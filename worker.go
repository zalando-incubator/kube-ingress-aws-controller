package main

import (
	"context"
	"errors"
	"math"
	"reflect"
	"sort"
	"time"

	"fmt"

	"runtime/debug"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	log "github.com/sirupsen/logrus"
	"github.com/zalando-incubator/kube-ingress-aws-controller/aws"
	"github.com/zalando-incubator/kube-ingress-aws-controller/certs"
	"github.com/zalando-incubator/kube-ingress-aws-controller/kubernetes"
	"github.com/zalando-incubator/kube-ingress-aws-controller/problem"
)

type worker struct {
	awsAdapter  *aws.Adapter
	kubeAdapter *kubernetes.Adapter
	metrics     *metrics

	certsProvider certs.CertificatesProvider
	certsPerALB   int
	certTTL       time.Duration

	globalWAFACL string

	cwAlarmConfig *kubernetes.ResourceLocation
}

type loadBalancer struct {
	ingresses                    map[string][]*kubernetes.Ingress
	scheme                       string
	stack                        *aws.Stack
	state                        *aws.LoadBalancerState
	existingStackCertificateARNs map[string]time.Time
	shared                       bool
	http2                        bool
	clusterLocal                 bool
	securityGroup                string
	sslPolicy                    string
	ipAddressType                string
	wafWebACLID                  string
	certTTL                      time.Duration
	cwAlarms                     aws.CloudWatchAlarmList
	loadBalancerType             string
}

const (
	ready int = iota
	update
	missing
	delete
)

const (
	cniEventRateLimit = 5 * time.Second
)

func (l *loadBalancer) Status() int {
	if l.clusterLocal {
		return ready
	}
	if l.stack.ShouldDelete() {
		return delete
	}
	if len(l.ingresses) != 0 && l.stack == nil {
		return missing
	}
	if firstRun || !l.inSync() && l.stack.IsComplete() {
		return update
	}
	return ready
}

// inSync checks if the loadBalancer is in sync with the backing CF stack. It's
// considered in sync when certs found for the ingresses match those already
// defined on the stack and the cloudwatch alarm config is up-to-date.
func (l *loadBalancer) inSync() bool {
	return reflect.DeepEqual(l.CertificateARNs(), l.stack.CertificateARNs) &&
		l.stack.CWAlarmConfigHash == l.cwAlarms.Hash() &&
		l.wafWebACLID == l.stack.WAFWebACLID
}

// addIngress adds an ingress object to the load balancer.
// The function returns true when the ingress was successfully added. The
// adding can fail in case the load balancer reached its limit of ingress
// certificates or if the scheme doesn't match.
func (l *loadBalancer) addIngress(certificateARNs []string, ingress *kubernetes.Ingress, maxCerts int) bool {
	if ingress.ClusterLocal {
		if ingresses, ok := l.ingresses[kubernetes.DefaultClusterLocalDomain]; ok {
			l.ingresses[kubernetes.DefaultClusterLocalDomain] = append(ingresses, ingress)
		} else {
			l.ingresses[kubernetes.DefaultClusterLocalDomain] = []*kubernetes.Ingress{ingress}
		}
		return true
	}

	resourceName := fmt.Sprintf("%s/%s", ingress.Namespace, ingress.Name)

	owner := ""
	if l.stack != nil {
		owner = l.stack.OwnerIngress
	}

	if owner != "" && resourceName != owner {
		return false
	}

	if !ingress.Shared && resourceName != owner {
		return false
	}

	// settings that would require a new load balancer no matter if it's
	// shared or not.
	if l.ipAddressType != ingress.IPAddressType ||
		l.scheme != ingress.Scheme ||
		l.loadBalancerType != ingress.LoadBalancerType ||
		l.http2 != ingress.HTTP2 {
		return false
	}

	// settings that can be changed on an existing load balancer if it's
	// NOT shared.
	if ingress.Shared && (l.securityGroup != ingress.SecurityGroup ||
		l.sslPolicy != ingress.SSLPolicy ||
		l.wafWebACLID != ingress.WAFWebACLID) {
		return false
	}

	// check if we can fit the ingress on the load balancer based on
	// maxCerts
	newCerts := 0
	for _, certificateARN := range certificateARNs {
		if _, ok := l.ingresses[certificateARN]; ok {
			continue
		}
		newCerts++
	}

	// if adding this ingress would result in more than maxCerts, then we
	// don't add the ingress
	if len(l.ingresses)+newCerts > maxCerts {
		return false
	}

	for _, certificateARN := range certificateARNs {
		l.ingresses[certificateARN] = append(l.ingresses[certificateARN], ingress)
	}

	l.shared = ingress.Shared
	return true
}

// CertificateARNs returns a map of certificates and their expiry times.
func (l *loadBalancer) CertificateARNs() map[string]time.Time {
	certificates := make(map[string]time.Time, len(l.ingresses))
	for arn, ingresses := range l.ingresses {
		// only include certificates required by at least one ingress.
		if len(ingresses) > 0 {
			certificates[arn] = time.Time{}
		}
	}

	for arn, ttl := range l.existingStackCertificateARNs {
		if _, ok := certificates[arn]; !ok {
			if ttl.IsZero() {
				certificates[arn] = time.Now().UTC().Add(l.certTTL)
			} else if ttl.After(time.Now().UTC()) {
				certificates[arn] = ttl
			}
		}
	}

	return certificates
}

// Owner returns the ingress resource owning the load balancer. If there are no
// owners it will return an empty string meaning the load balancer is shared
// between multiple ingresses.
func (l *loadBalancer) Owner() string {
	if l.shared {
		return ""
	}

	for _, ingresses := range l.ingresses {
		for _, ingress := range ingresses {
			return fmt.Sprintf("%s/%s", ingress.Namespace, ingress.Name)
		}
	}

	return ""
}

// CertificatesFinder interface represents a list of certificates
// and some basic operations than can be performed on them.
type CertificatesFinder interface {
	CertificateSummaries() []*certs.CertificateSummary
	CertificateExists(certificateARN string) bool
	FindMatchingCertificateIDs([]string) []string
}

// Certificates represents a generic list of certificates
type Certificates struct {
	certARNSet           map[string]struct{}
	certificateSummaries []*certs.CertificateSummary
}

func NewCertificates(certs []*certs.CertificateSummary) *Certificates {
	certARNSet := make(map[string]struct{}, len(certs))
	for _, cert := range certs {
		certARNSet[cert.ID()] = struct{}{}
	}

	return &Certificates{
		certARNSet:           certARNSet,
		certificateSummaries: certs,
	}
}

// CertificateSummaries returns summaries of all certificates
func (c *Certificates) CertificateSummaries() []*certs.CertificateSummary {
	return c.certificateSummaries
}

// CertificateExists checks if certificate with given ARN/ID is present in the collection
func (c *Certificates) CertificateExists(arn string) bool {
	_, ok := c.certARNSet[arn]
	return ok
}

// FindMatchingCertificateIDs get IDs of all certificates matching to given hostnames
func (c *Certificates) FindMatchingCertificateIDs(hostnames []string) []string {
	certificateSummaries := certs.FindBestMatchingCertificates(c.certificateSummaries, hostnames)
	certIDs := make([]string, 0, len(certificateSummaries))
	for _, cert := range certificateSummaries {
		certIDs = append(certIDs, cert.ID())
	}

	return certIDs
}

func (w *worker) startPolling(ctx context.Context, pollingInterval time.Duration) {
	for {
		if errs := w.doWork(ctx).Errors(); len(errs) > 0 {
			for _, err := range errs {
				log.Error(err)
			}
		} else {
			w.metrics.lastSyncTimestamp.SetToCurrentTime()
		}
		firstRun = false

		log.Debugf("Start polling sleep %s", pollingInterval)
		select {
		case <-time.After(pollingInterval):
		case <-ctx.Done():
			return
		}
	}
}

func (w *worker) doWork(ctx context.Context) (problems *problem.List) {
	problems = new(problem.List)
	defer func() {
		if r := recover(); r != nil {
			debug.PrintStack()
			problems.Add("panic caused by: %v", r)
		}
	}()

	ingresses, err := w.kubeAdapter.ListResources()
	if err != nil {
		return problems.Add("failed to list ingress resources: %w", err)
	}

	stacks, err := w.awsAdapter.FindManagedStacks(ctx)
	if err != nil {
		return problems.Add("failed to list managed stacks: %w", err)
	}

	for _, stack := range stacks {
		if err := stack.Err(); err != nil {
			problems.Add("stack %s error: %w", stack.Name, err)
		}
	}

	stackELBs, err := w.awsAdapter.GetStackLBStates(ctx, stacks)
	if err != nil {
		return problems.Add("failed to get stack ELBs: %w", err)
	}

	err = w.awsAdapter.UpdateAutoScalingGroupsAndInstances(ctx)
	if err != nil {
		return problems.Add("failed to get instances from EC2: %w", err)
	}

	certificateSummaries, err := w.certsProvider.GetCertificates(ctx)
	if err != nil {
		return problems.Add("failed to get certificates: %w", err)
	}

	if len(certificateSummaries) == 0 {
		return problems.Add("no certificates found")
	}

	cwAlarms, err := w.getCloudWatchAlarms()
	if err != nil {
		return problems.Add("failed to retrieve cloudwatch alarm configuration: %w", err)
	}

	counts := countByIngressType(ingresses)

	w.metrics.ingressesTotal.Set(float64(counts[kubernetes.TypeIngress]))
	w.metrics.routegroupsTotal.Set(float64(counts[kubernetes.TypeRouteGroup]))
	w.metrics.stacksTotal.Set(float64(len(stacks)))
	w.metrics.ownedAutoscalingGroupsTotal.Set(float64(len(w.awsAdapter.OwnedAutoScalingGroups)))
	w.metrics.targetedAutoscalingGroupsTotal.Set(float64(len(w.awsAdapter.TargetedAutoScalingGroups)))
	w.metrics.instancesTotal.Set(float64(w.awsAdapter.CachedInstances()))
	w.metrics.standaloneInstancesTotal.Set(float64(len(w.awsAdapter.SingleInstances())))
	w.metrics.certificatesTotal.Set(float64(len(certificateSummaries)))
	w.metrics.cloudWatchAlarmsTotal.Set(float64(len(cwAlarms)))

	log.Debugf("Found %d ingress(es)", counts[kubernetes.TypeIngress])
	log.Debugf("Found %d route group(s)", counts[kubernetes.TypeRouteGroup])
	log.Debugf("Found %d stack(s)", len(stacks))
	log.Debugf("Found %d owned auto scaling group(s)", len(w.awsAdapter.OwnedAutoScalingGroups))
	log.Debugf("Found %d targeted auto scaling group(s)", len(w.awsAdapter.TargetedAutoScalingGroups))
	log.Debugf("Found %d EC2 instance(s)", w.awsAdapter.CachedInstances())
	log.Debugf("Found %d single instance(s)", len(w.awsAdapter.SingleInstances()))
	log.Debugf("Found %d certificate(s)", len(certificateSummaries))
	log.Debugf("Found %d cloudwatch alarm configuration(s)", len(cwAlarms))

	w.awsAdapter.UpdateTargetGroupsAndAutoScalingGroups(ctx, stacks, problems)

	certs := NewCertificates(certificateSummaries)
	model := buildManagedModel(certs, w.certsPerALB, w.certTTL, ingresses, stackELBs, cwAlarms, w.globalWAFACL)
	log.Debugf("Have %d model(s)", len(model))
	for _, loadBalancer := range model {
		switch loadBalancer.Status() {
		case delete:
			w.deleteStack(ctx, loadBalancer, problems)
		case missing:
			w.createStack(ctx, loadBalancer, problems)
			w.updateIngress(loadBalancer, problems)
		case ready:
			w.updateIngress(loadBalancer, problems)
		case update:
			w.updateStack(ctx, loadBalancer, problems)
			w.updateIngress(loadBalancer, problems)
		}
	}
	return
}

func countByIngressType(ingresses []*kubernetes.Ingress) map[kubernetes.IngressType]int {
	counts := make(map[kubernetes.IngressType]int)
	for _, ing := range ingresses {
		counts[ing.ResourceType]++
	}
	return counts
}

func sortStacks(stackLBState []*aws.StackLBState) {
	sort.Slice(stackLBState, func(i, j int) bool {
		if len(stackLBState[i].Stack.CertificateARNs) == len(stackLBState[j].Stack.CertificateARNs) {
			return stackLBState[i].Stack.Name < stackLBState[j].Stack.Name
		}
		return len(stackLBState[i].Stack.CertificateARNs) > len(stackLBState[j].Stack.CertificateARNs)
	})
}

func getAllLoadBalancers(certs CertificatesFinder, certTTL time.Duration, stackLBStates []*aws.StackLBState) []*loadBalancer {
	loadBalancers := make([]*loadBalancer, 0, len(stackLBStates))

	for _, sl := range stackLBStates {
		lb := &loadBalancer{
			stack:                        sl.Stack,
			state:                        sl.LBState,
			existingStackCertificateARNs: make(map[string]time.Time, len(sl.Stack.CertificateARNs)),
			ingresses:                    make(map[string][]*kubernetes.Ingress),
			scheme:                       sl.Stack.Scheme,
			shared:                       sl.Stack.OwnerIngress == "",
			securityGroup:                sl.Stack.SecurityGroup,
			sslPolicy:                    sl.Stack.SSLPolicy,
			ipAddressType:                sl.Stack.IpAddressType,
			loadBalancerType:             sl.Stack.LoadBalancerType,
			http2:                        sl.Stack.HTTP2,
			wafWebACLID:                  sl.Stack.WAFWebACLID,
			certTTL:                      certTTL,
		}
		// initialize ingresses map with existing certificates from the stack.
		// Also filter the stack certificates so we have a set of
		// certificates which are still availale, compared to what was
		// latest stored on the stack.
		for cert, expiry := range sl.Stack.CertificateARNs {
			if certs.CertificateExists(cert) {
				lb.existingStackCertificateARNs[cert] = expiry
				lb.ingresses[cert] = make([]*kubernetes.Ingress, 0)
			}
		}
		loadBalancers = append(loadBalancers, lb)
	}

	return loadBalancers
}

func matchIngressesToLoadBalancers(
	loadBalancers []*loadBalancer,
	certs CertificatesFinder,
	certsPerALB int,
	ingresses []*kubernetes.Ingress,
) []*loadBalancer {
	clusterLocalLB := &loadBalancer{
		clusterLocal: true,
		ingresses:    make(map[string][]*kubernetes.Ingress),
	}
	loadBalancers = append(loadBalancers, clusterLocalLB)

	for _, ingress := range ingresses {
		if ingress.ClusterLocal {
			clusterLocalLB.addIngress(nil, ingress, math.MaxInt64)
			continue
		}

		var certificateARNs []string

		if ingress.CertificateARN != "" {
			if !certs.CertificateExists(ingress.CertificateARN) {
				log.Errorf("Failed to find certificate %s for %s", ingress.CertificateARN, ingress)
				continue
			}
			certificateARNs = []string{ingress.CertificateARN}
		} else {
			certificateARNs = certs.FindMatchingCertificateIDs(ingress.Hostnames)
			if len(certificateARNs) == 0 {
				log.Errorf("No certificates found for hostnames %v of %s", ingress.Hostnames, ingress)
				continue
			}
		}

		// try to add ingress to existing ALB stacks until certificate
		// limit is exeeded.
		added := false
		for _, lb := range loadBalancers {
			// TODO(mlarsen): hack to phase out old load balancers
			// which can't be updated to include type
			// specification.
			// Can be removed in a later version
			supportedLBType := lb.loadBalancerType == aws.LoadBalancerTypeApplication ||
				lb.loadBalancerType == aws.LoadBalancerTypeNetwork
			if !supportedLBType {
				continue
			}

			if lb.addIngress(certificateARNs, ingress, certsPerALB) {
				added = true
				break
			}
		}

		// if the ingress was not added to the ALB stack because of
		// non-matching scheme, non-matching security group or too many certificates, add a new
		// stack.
		if !added {
			i := make(map[string][]*kubernetes.Ingress, len(certificateARNs))
			for _, certificateARN := range certificateARNs {
				i[certificateARN] = []*kubernetes.Ingress{ingress}
			}
			loadBalancers = append(
				loadBalancers,
				&loadBalancer{
					ingresses:        i,
					scheme:           ingress.Scheme,
					shared:           ingress.Shared,
					securityGroup:    ingress.SecurityGroup,
					sslPolicy:        ingress.SSLPolicy,
					ipAddressType:    ingress.IPAddressType,
					loadBalancerType: ingress.LoadBalancerType,
					http2:            ingress.HTTP2,
					wafWebACLID:      ingress.WAFWebACLID,
				},
			)
		}
	}

	return loadBalancers
}

// addCloudWatchAlarms attaches CloudWatch Alarms to each load balancer model
// in the list. It ensures that the alarm config is copied so that it can be
// adjusted safely for each load balancer.
func attachCloudWatchAlarms(loadBalancers []*loadBalancer, cwAlarms aws.CloudWatchAlarmList) {
	for _, loadBalancer := range loadBalancers {
		lbAlarms := make(aws.CloudWatchAlarmList, len(cwAlarms))

		copy(lbAlarms, cwAlarms)

		loadBalancer.cwAlarms = lbAlarms
	}
}

func attachGlobalWAFACL(ings []*kubernetes.Ingress, globalWAFACL string) {
	for _, ing := range ings {
		if ing.WAFWebACLID != "" {
			continue
		}

		ing.WAFWebACLID = globalWAFACL
	}
}

func buildManagedModel(
	certs CertificatesFinder,
	certsPerALB int,
	certTTL time.Duration,
	ingresses []*kubernetes.Ingress,
	stackLBStates []*aws.StackLBState,
	cwAlarms aws.CloudWatchAlarmList,
	globalWAFACL string,
) []*loadBalancer {
	sortStacks(stackLBStates)
	attachGlobalWAFACL(ingresses, globalWAFACL)
	model := getAllLoadBalancers(certs, certTTL, stackLBStates)
	model = matchIngressesToLoadBalancers(model, certs, certsPerALB, ingresses)
	attachCloudWatchAlarms(model, cwAlarms)

	return model
}

func (w *worker) createStack(ctx context.Context, lb *loadBalancer, problems *problem.List) {
	certificates := make([]string, 0, len(lb.ingresses))
	for cert := range lb.ingresses {
		certificates = append(certificates, cert)
	}

	log.Infof("Creating stack for certificates %q / ingress %q", certificates, lb.ingresses)

	stackId, err := w.awsAdapter.CreateStack(ctx, certificates, lb.scheme, lb.securityGroup, lb.Owner(), lb.sslPolicy, lb.ipAddressType, lb.wafWebACLID, lb.cwAlarms, lb.loadBalancerType, lb.http2)
	if err != nil {
		if isAlreadyExistsError(err) {
			lb.stack, err = w.awsAdapter.GetStack(ctx, stackId)
			if err == nil {
				return
			}
		}
		problems.Add("failed to create stack %q: %w", certificates, err)
	} else {
		w.metrics.changesTotal.created("stack")
		log.Infof("Stack %q for certificates %q created", stackId, certificates)
	}
}

func (w *worker) updateStack(ctx context.Context, lb *loadBalancer, problems *problem.List) {
	certificates := lb.CertificateARNs()

	log.Infof("Updating %q stack for %d certificates / %d ingresses", lb.scheme, len(certificates), len(lb.ingresses))

	stackId, err := w.awsAdapter.UpdateStack(ctx, lb.stack.Name, certificates, lb.scheme, lb.securityGroup, lb.Owner(), lb.sslPolicy, lb.ipAddressType, lb.wafWebACLID, lb.cwAlarms, lb.loadBalancerType, lb.http2)
	if isNoUpdatesToBePerformedError(err) {
		log.Debugf("Stack(%q) is already up to date", certificates)
	} else if err != nil {
		problems.Add("failed to update stack %q: %w", certificates, err)
	} else {
		w.metrics.changesTotal.updated("stack")
		log.Infof("Stack %q for certificate %q updated", stackId, certificates)
	}
}

func isAlreadyExistsError(err error) bool {
	var alreadyExistsErr *types.AlreadyExistsException
	if err != nil && errors.As(err, &alreadyExistsErr) {
		return true
	}
	return false
}

func isNoUpdatesToBePerformedError(err error) bool {
	if err != nil {
		return strings.Contains(err.Error(), "No updates are to be performed")
	}
	return false
}

func (w *worker) updateIngress(lb *loadBalancer, problems *problem.List) {
	var dnsName string
	if lb.clusterLocal {
		dnsName = kubernetes.DefaultClusterLocalDomain
	} else {
		// only update ingress if the CF stack is in a completed state and the ELB
		// is in the active state.
		if lb.stack == nil {
			log.Infof("CF stack is nil, skipping ingress update")
			return
		}
		if !lb.stack.IsComplete() {
			log.Infof(
				"CF stack %q is not in a completed state, skipping ingress update",
				lb.stack.Name,
			)
			return
		}
		if !aws.IsActiveLBState(lb.state) {
			log.Infof(
				"The load balancer of CF stack %q is not in active state (state: %s, lb-ARN: %s), skipping ingress update",
				lb.stack.Name, aws.GetLBStateString(lb.state), lb.stack.LoadBalancerARN,
			)
			return
		}
		dnsName = strings.ToLower(lb.stack.DNSName) // lower case to satisfy Kubernetes reqs
	}
	for _, ingresses := range lb.ingresses {
		for _, ing := range ingresses {
			if err := w.kubeAdapter.UpdateIngressLoadBalancer(ing, dnsName); err != nil {
				if err == kubernetes.ErrUpdateNotNeeded {
					log.Debugf("Update not needed for %s with DNS name %s", ing, dnsName)
				} else {
					problems.Add("failed to update %s: %w", ing, err)
				}
			} else {
				w.metrics.changesTotal.updated(string(ing.ResourceType))
				log.Infof("Updated %s with DNS name %s", ing, dnsName)
			}
		}
	}
}

func (w *worker) deleteStack(ctx context.Context, lb *loadBalancer, problems *problem.List) {
	stackName := lb.stack.Name
	if err := w.awsAdapter.DeleteStack(ctx, lb.stack); err != nil {
		problems.Add("failed to delete stack %q: %w", stackName, err)
	} else {
		w.metrics.changesTotal.deleted("stack")
		log.Infof("Deleted orphaned stack %q", stackName)
	}
}

// getCloudWatchAlarms retrieves CloudWatch Alarm configuration from a
// ConfigMap described by [worker.cwAlarmConfig]. If [worker.cwAlarmConfig] is nil, an empty alarm
// configuration will be returned. Returns any error that might occur while
// retrieving the configuration.
func (w *worker) getCloudWatchAlarms() (aws.CloudWatchAlarmList, error) {
	if w.cwAlarmConfig == nil {
		return aws.CloudWatchAlarmList{}, nil
	}

	configMap, err := w.kubeAdapter.GetConfigMap(w.cwAlarmConfig.Namespace, w.cwAlarmConfig.Name)
	if err != nil {
		return nil, err
	}

	return getCloudWatchAlarmsFromConfigMap(configMap), nil
}

// getCloudWatchAlarmsFromConfigMap extracts cloudwatch alarm configuration
// from ConfigMap data. It will collect alarm configuration from all ConfigMap
// data keys it finds. If a ConfigMap data key contains invalid data, an error
// is logged and the key will be ignored. The sort order of the resulting slice
// is guaranteed to be stable.
func getCloudWatchAlarmsFromConfigMap(configMap *kubernetes.ConfigMap) aws.CloudWatchAlarmList {
	configList := aws.CloudWatchAlarmList{}

	keys := make([]string, 0, len(configMap.Data))
	for k := range configMap.Data {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	for _, key := range keys {
		data := []byte(configMap.Data[key])

		list, err := aws.NewCloudWatchAlarmListFromYAML(data)
		if err != nil {
			log.Warnf("Ignoring cloudwatch alarm configuration from config map key %q due to error: %v", key, err)
			continue
		}

		configList = append(configList, list...)
	}

	return configList
}

// cniEventHandler syncronizes the events from kubernetes and the status updates from the load balancer controller.
// Events updates a rate limited.
func cniEventHandler(ctx context.Context, targetCNIcfg *aws.TargetCNIconfig,
	targetSetter func(context.Context, []string, []string) error, informer func(context.Context, chan<- []string) error) {
	log.Infoln("Starting CNI event handler")

	rateLimiter := time.NewTicker(cniEventRateLimit)
	defer rateLimiter.Stop()

	endpointCh := make(chan []string, 10)
	go func() {
		err := informer(ctx, endpointCh)
		if err != nil {
			log.Errorf("Informer failed: %v", err)
			return
		}
	}()

	var cniTargetGroupARNs, endpoints []string
	for {
		select {
		case <-ctx.Done():
			return
		case cniTargetGroupARNs = <-targetCNIcfg.TargetGroupCh:
			log.Debugf("new message target groups: %v", cniTargetGroupARNs)
		case endpoints = <-endpointCh:
			log.Debugf("new message endpoints: %v", endpoints)
		}

		// prevent cleanup due to startup inconsistenty, arns and endpoints can be empty but never nil
		if cniTargetGroupARNs == nil || endpoints == nil {
			continue
		}
		if len(endpointCh) > 0 || len(targetCNIcfg.TargetGroupCh) > 0 {
			log.Debugf("flushing, messages queued: %d:%d", len(endpointCh), len(cniTargetGroupARNs))
			continue
		}
		<-rateLimiter.C
		err := targetSetter(ctx, endpoints, cniTargetGroupARNs)
		if err != nil {
			log.Error(err)
		}
	}
}
