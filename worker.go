package main

import (
	"context"
	"math"
	"reflect"
	"sort"
	"time"

	"fmt"

	"runtime/debug"
	"strings"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/zalando-incubator/kube-ingress-aws-controller/aws"
	"github.com/zalando-incubator/kube-ingress-aws-controller/certs"
	"github.com/zalando-incubator/kube-ingress-aws-controller/kubernetes"
)

type loadBalancer struct {
	ingresses        map[string][]*kubernetes.Ingress
	scheme           string
	stack            *aws.Stack
	shared           bool
	http2            bool
	clusterLocal     bool
	securityGroup    string
	sslPolicy        string
	ipAddressType    string
	wafWebACLId      string
	certTTL          time.Duration
	cwAlarms         aws.CloudWatchAlarmList
	loadBalancerType string
}

const (
	ready int = iota
	update
	missing
	delete
)

const (
	maxTargetGroupSupported = 1000
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
		l.wafWebACLId == l.stack.WAFWebACLId
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

	if l.ipAddressType != ingress.IPAddressType ||
		l.scheme != ingress.Scheme ||
		l.securityGroup != ingress.SecurityGroup ||
		l.sslPolicy != ingress.SSLPolicy ||
		l.loadBalancerType != ingress.LoadBalancerType ||
		l.http2 != ingress.HTTP2 {
		return false
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
	if !ingress.ClusterLocal {
		l.wafWebACLId = ingress.WAFWebACLId
	}

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

	for arn, ttl := range l.stack.CertificateARNs {
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
	certificateSummaries []*certs.CertificateSummary
}

// CertificateSummaries returns summaries of all certificates
func (c *Certificates) CertificateSummaries() []*certs.CertificateSummary {
	return c.certificateSummaries
}

// CertificateExists checks if certificate with given ARN/ID is present in the collection
func (c *Certificates) CertificateExists(arn string) bool {
	for _, cert := range c.certificateSummaries {
		if arn == cert.ID() {
			return true
		}
	}
	return false
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

func startPolling(ctx context.Context, certsProvider certs.CertificatesProvider, certsPerALB int, certTTL time.Duration, awsAdapter *aws.Adapter, kubeAdapter *kubernetes.Adapter, pollingInterval time.Duration) {
	for {
		if err := doWork(certsProvider, certsPerALB, certTTL, awsAdapter, kubeAdapter); err != nil {
			log.Error(err)
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

func doWork(certsProvider certs.CertificatesProvider, certsPerALB int, certTTL time.Duration, awsAdapter *aws.Adapter, kubeAdapter *kubernetes.Adapter) error {
	defer func() error {
		if r := recover(); r != nil {
			log.Errorln("shit has hit the fan:", errors.Wrap(r.(error), "panic caused by"))
			debug.PrintStack()
			return r.(error)
		}
		return nil
	}()

	ingresses, err := kubeAdapter.ListResources()
	if err != nil {
		return fmt.Errorf("doWork failed to list ingress resources: %v", err)
	}
	log.Infof("Found %d ingress(es)", len(ingresses))

	stacks, err := awsAdapter.FindManagedStacks()
	if err != nil {
		return fmt.Errorf("doWork failed to list managed stacks: %v", err)
	}
	log.Infof("Found %d stack(s)", len(stacks))

	err = awsAdapter.UpdateAutoScalingGroupsAndInstances()
	if err != nil {
		return fmt.Errorf("doWork failed to get instances from EC2: %v", err)
	}

	certificateSummaries, err := certsProvider.GetCertificates()
	if err != nil {
		return fmt.Errorf("doWork failed to get certificates: %v", err)
	}

	cwAlarms, err := getCloudWatchAlarms(kubeAdapter, cwAlarmConfigMapLocation)
	if err != nil {
		return fmt.Errorf("doWork failed to retrieve cloudwatch alarm configuration: %v", err)
	}

	awsAdapter.UpdateTargetGroupsAndAutoScalingGroups(stacks)
	log.Infof("Found %d owned auto scaling group(s)", len(awsAdapter.OwnedAutoScalingGroups))
	log.Infof("Found %d targeted auto scaling group(s)", len(awsAdapter.TargetedAutoScalingGroups))
	log.Infof("Found %d single instance(s)", len(awsAdapter.SingleInstances()))
	log.Infof("Found %d EC2 instance(s)", awsAdapter.CachedInstances())
	log.Infof("Found %d certificate(s)", len(certificateSummaries))
	log.Infof("Found %d cloudwatch alarm configuration(s)", len(cwAlarms))

	certs := &Certificates{certificateSummaries: certificateSummaries}
	model := buildManagedModel(certs, certsPerALB, certTTL, ingresses, stacks, cwAlarms)
	log.Debugf("Have %d model(s)", len(model))
	for _, loadBalancer := range model {
		switch loadBalancer.Status() {
		case delete:
			deleteStack(awsAdapter, loadBalancer)
		case missing:
			createStack(awsAdapter, loadBalancer)
			updateIngress(kubeAdapter, loadBalancer)
		case ready:
			updateIngress(kubeAdapter, loadBalancer)
		case update:
			updateStack(awsAdapter, loadBalancer)
			updateIngress(kubeAdapter, loadBalancer)
		}
	}

	return nil
}

func sortStacks(stacks []*aws.Stack) {
	sort.Slice(stacks, func(i, j int) bool {
		if len(stacks[i].CertificateARNs) == len(stacks[j].CertificateARNs) {
			return stacks[i].Name < stacks[j].Name
		}
		return len(stacks[i].CertificateARNs) > len(stacks[j].CertificateARNs)
	})
}

func getAllLoadBalancers(certTTL time.Duration, stacks []*aws.Stack) []*loadBalancer {
	loadBalancers := make([]*loadBalancer, 0, len(stacks))

	for _, stack := range stacks {
		lb := &loadBalancer{
			stack:            stack,
			ingresses:        make(map[string][]*kubernetes.Ingress),
			scheme:           stack.Scheme,
			shared:           stack.OwnerIngress == "",
			securityGroup:    stack.SecurityGroup,
			sslPolicy:        stack.SSLPolicy,
			ipAddressType:    stack.IpAddressType,
			loadBalancerType: stack.LoadBalancerType,
			http2:            stack.HTTP2,
			certTTL:          certTTL,
		}
		// initialize ingresses map with existing certificates from the
		// stack.
		for cert := range stack.CertificateARNs {
			lb.ingresses[cert] = make([]*kubernetes.Ingress, 0)
		}
		loadBalancers = append(loadBalancers, lb)
	}

	return loadBalancers
}

func groupLBsByWAF(lbs []*loadBalancer) map[string][]*loadBalancer {
	m := make(map[string][]*loadBalancer)
	for _, lb := range lbs {
		var group string
		if lb.stack != nil {
			group = lb.stack.WAFWebACLId
		}

		m[group] = append(m[group], lb)
	}

	return m
}

func flattenLBs(lbs map[string][]*loadBalancer) []*loadBalancer {
	var flat []*loadBalancer
	for _, group := range lbs {
		for _, lb := range group {
			flat = append(flat, lb)
		}
	}

	return flat
}

func matchIngressesToLoadBalancers(loadBalancers []*loadBalancer, certs CertificatesFinder, certsPerALB int, ingresses []*kubernetes.Ingress) []*loadBalancer {
	clusterLocalLB := &loadBalancer{
		clusterLocal: true,
		ingresses:    make(map[string][]*kubernetes.Ingress),
	}
	loadBalancers = append(loadBalancers, clusterLocalLB)

	lbsByWAF := groupLBsByWAF(loadBalancers)
	for _, ingress := range ingresses {
		if ingress.ClusterLocal {
			clusterLocalLB.addIngress(nil, ingress, math.MaxInt64)
			continue
		}

		var certificateARNs []string

		if ingress.CertificateARN != "" {
			if !certs.CertificateExists(ingress.CertificateARN) {
				log.Errorf("Failed to find certificate '%s' for ingress '%s/%s'", ingress.CertificateARN, ingress.Namespace, ingress.Name)
				continue
			}
			certificateARNs = []string{ingress.CertificateARN}
		} else {
			certificateARNs = certs.FindMatchingCertificateIDs(ingress.Hostnames)
			if len(certificateARNs) == 0 {
				log.Errorf("No certificates found for %v", ingress.Hostnames)
				continue
			}
		}

		// try to add ingress to existing ALB stacks until certificate
		// limit is exeeded.
		added := false
		for _, lb := range lbsByWAF[ingress.WAFWebACLId] {
			// TODO(mlarsen): hack to phase out old load balancers
			// which can't be updated to include type
			// specification.
			// Can be removed in a later version
			if lb.loadBalancerType != aws.LoadBalancerTypeApplication && lb.loadBalancerType != aws.LoadBalancerTypeNetwork {
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
			lbsByWAF[ingress.WAFWebACLId] = append(
				lbsByWAF[ingress.WAFWebACLId],
				&loadBalancer{
					ingresses:        i,
					scheme:           ingress.Scheme,
					shared:           ingress.Shared,
					securityGroup:    ingress.SecurityGroup,
					sslPolicy:        ingress.SSLPolicy,
					ipAddressType:    ingress.IPAddressType,
					loadBalancerType: ingress.LoadBalancerType,
					http2:            ingress.HTTP2,
					wafWebACLId:      ingress.WAFWebACLId,
				},
			)
		}
	}

	return flattenLBs(lbsByWAF)
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

func buildManagedModel(certs CertificatesFinder, certsPerALB int, certTTL time.Duration, ingresses []*kubernetes.Ingress, stacks []*aws.Stack, cwAlarms aws.CloudWatchAlarmList) []*loadBalancer {
	sortStacks(stacks)
	model := getAllLoadBalancers(certTTL, stacks)
	model = matchIngressesToLoadBalancers(model, certs, certsPerALB, ingresses)
	attachCloudWatchAlarms(model, cwAlarms)

	return model
}

func createStack(awsAdapter *aws.Adapter, lb *loadBalancer) {
	certificates := make([]string, 0, len(lb.ingresses))
	for cert := range lb.ingresses {
		certificates = append(certificates, cert)
	}

	log.Infof("creating stack for certificates %q / ingress %q", certificates, lb.ingresses)

	stackId, err := awsAdapter.CreateStack(certificates, lb.scheme, lb.securityGroup, lb.Owner(), lb.sslPolicy, lb.ipAddressType, lb.wafWebACLId, lb.cwAlarms, lb.loadBalancerType, lb.http2)
	if err != nil {
		if isAlreadyExistsError(err) {
			lb.stack, err = awsAdapter.GetStack(stackId)
			if err == nil {
				return
			}
		}
		log.Errorf("createStack(%q) failed: %v", certificates, err)
	} else {
		log.Infof("stack %q for certificates %q created", stackId, certificates)
	}
}

func updateStack(awsAdapter *aws.Adapter, lb *loadBalancer) {
	certificates := lb.CertificateARNs()

	log.Infof("updating %q stack for %d certificates / %d ingresses", lb.scheme, len(certificates), len(lb.ingresses))

	stackId, err := awsAdapter.UpdateStack(lb.stack.Name, certificates, lb.scheme, lb.sslPolicy, lb.ipAddressType, lb.wafWebACLId, lb.cwAlarms, lb.loadBalancerType, lb.http2)
	if isNoUpdatesToBePerformedError(err) {
		log.Debugf("stack(%q) is already up to date", certificates)
	} else if err != nil {
		log.Errorf("updateStack(%q) failed: %v", certificates, err)
	} else {
		log.Infof("stack %q for certificate %q updated", stackId, certificates)
	}
}

func isAlreadyExistsError(err error) bool {
	if awsErr, ok := err.(awserr.Error); ok {
		return awsErr.Code() == cloudformation.ErrCodeAlreadyExistsException
	}
	return false
}

func isNoUpdatesToBePerformedError(err error) bool {
	if err == nil {
		return false
	}
	if _, ok := err.(awserr.Error); ok {
		return strings.Contains(err.Error(), "No updates are to be performed")
	}
	return false
}

func updateIngress(kubeAdapter *kubernetes.Adapter, lb *loadBalancer) {
	var dnsName string
	if lb.clusterLocal {
		dnsName = kubernetes.DefaultClusterLocalDomain
	} else {
		// only update ingress if the stack exists and is in a complete state.
		if lb.stack == nil || !lb.stack.IsComplete() {
			return
		}
		dnsName = strings.ToLower(lb.stack.DNSName) // lower case to satisfy Kubernetes reqs
	}
	for _, ingresses := range lb.ingresses {
		for _, ing := range ingresses {
			if err := kubeAdapter.UpdateIngressLoadBalancer(ing, dnsName); err != nil {
				if err == kubernetes.ErrUpdateNotNeeded {
					log.Debugf("Ingress update not needed %v with DNS name %q", ing, dnsName)
				} else {
					log.Errorf("Failed to update ingress: %v", err)
				}
			} else {
				log.Infof("updated ingress %v with DNS name %q", ing, dnsName)
			}
		}
	}
}

func deleteStack(awsAdapter *aws.Adapter, lb *loadBalancer) {
	stackName := lb.stack.Name
	if err := awsAdapter.DeleteStack(lb.stack); err != nil {
		log.Errorf("deleteStack failed to delete stack %q: %v", stackName, err)
	} else {
		log.Infof("deleted orphaned stack %q", stackName)
	}
}

// getCloudWatchAlarms retrieves CloudWatch Alarm configuration from a
// ConfigMap described by configMapLoc. If configMapLoc is nil, an empty alarm
// configuration will be returned. Returns any error that might occur while
// retrieving the configuration.
func getCloudWatchAlarms(kubeAdapter *kubernetes.Adapter, configMapLoc *kubernetes.ResourceLocation) (aws.CloudWatchAlarmList, error) {
	if configMapLoc == nil {
		return aws.CloudWatchAlarmList{}, nil
	}

	configMap, err := kubeAdapter.GetConfigMap(configMapLoc.Namespace, configMapLoc.Name)
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
			log.Warnf("ignoring cloudwatch alarm configuration from config map key %q due to error: %v", key, err)
			continue
		}

		configList = append(configList, list...)
	}

	return configList
}
