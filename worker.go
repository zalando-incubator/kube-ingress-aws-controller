package main

import (
	"log"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"time"

	"fmt"

	"runtime/debug"
	"strings"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/pkg/errors"
	"github.com/zalando-incubator/kube-ingress-aws-controller/aws"
	"github.com/zalando-incubator/kube-ingress-aws-controller/certs"
	"github.com/zalando-incubator/kube-ingress-aws-controller/kubernetes"
)

type loadBalancer struct {
	ingresses map[string][]*kubernetes.Ingress
	scheme    string
	stack     *aws.Stack
	shared    bool
	certTTL   time.Duration
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
	if l.stack.ShouldDelete() {
		return delete
	}
	if len(l.ingresses) != 0 && l.stack == nil {
		return missing
	}
	if !l.certsEqual() && l.stack.IsComplete() {
		return update
	}
	return ready
}

// certsEqual checks if the certs found for the ingresses match those already
// defined on the LB stack.
func (l *loadBalancer) certsEqual() bool {
	if len(l.ingresses) != len(l.stack.CertificateARNs) {
		return false
	}

	for arn, _ := range l.ingresses {
		if ttl, ok := l.stack.CertificateARNs[arn]; !ok || !ttl.IsZero() {
			return false
		}
	}
	return true
}

// AddIngress adds an ingress object to the load balancer.
// The function returns true when the ingress was successfully added. The
// adding can fail in case the load balancer reached its limit of ingress
// certificates or if the scheme doesn't match.
func (l *loadBalancer) AddIngress(certificateARNs []string, ingress *kubernetes.Ingress, maxCerts int) bool {
	if l.scheme != ingress.Scheme {
		return false
	}

	resourceName := fmt.Sprintf("%s/%s", ingress.Namespace, ingress.Name)

	owner := ""
	if l.stack != nil {
		owner = l.stack.OwnerIngress
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

	// if adding this ingress would result in more than maxCerts then we
	// don't add the ingress
	if len(l.ingresses)+newCerts > maxCerts {
		return false
	}

	for _, certificateARN := range certificateARNs {
		if ingresses, ok := l.ingresses[certificateARN]; ok {
			l.ingresses[certificateARN] = append(ingresses, ingress)
		} else {
			l.ingresses[certificateARN] = []*kubernetes.Ingress{ingress}
		}
	}

	l.shared = ingress.Shared

	return true
}

// CertificateARNs returns a map of certificates and their expiry times.
func (l *loadBalancer) CertificateARNs() map[string]time.Time {
	certificates := make(map[string]time.Time, len(l.ingresses))
	for arn := range l.ingresses {
		certificates[arn] = time.Time{}
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

func waitForTerminationSignals(signals ...os.Signal) chan os.Signal {
	c := make(chan os.Signal, 1)
	signal.Notify(c, signals...)
	return c
}

func startPolling(quitCH chan struct{}, certsProvider certs.CertificatesProvider, certsPerALB int, certTTL time.Duration, awsAdapter *aws.Adapter, kubeAdapter *kubernetes.Adapter, pollingInterval time.Duration) {
	for {
		log.Printf("Start polling sleep %s", pollingInterval)
		select {
		case <-waitForTerminationSignals(syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT):
			quitCH <- struct{}{}
			return
		case <-time.After(pollingInterval):
			if err := doWork(certsProvider, certsPerALB, certTTL, awsAdapter, kubeAdapter); err != nil {
				log.Println(err)
			}
		}
	}
}

func doWork(certsProvider certs.CertificatesProvider, certsPerALB int, certTTL time.Duration, awsAdapter *aws.Adapter, kubeAdapter *kubernetes.Adapter) error {
	defer func() error {
		if r := recover(); r != nil {
			log.Println("shit has hit the fan:", errors.Wrap(r.(error), "panic caused by"))
			debug.PrintStack()
			return r.(error)
		}
		return nil
	}()

	ingresses, err := kubeAdapter.ListIngress()
	if err != nil {
		return fmt.Errorf("doWork failed to list ingress resources: %v", err)
	}
	log.Printf("Found %d ingress(es)", len(ingresses))

	stacks, err := awsAdapter.FindManagedStacks()
	if err != nil {
		return fmt.Errorf("doWork failed to list managed stacks: %v", err)
	}
	log.Printf("Found %d stack(s)", len(stacks))

	err = awsAdapter.UpdateAutoScalingGroupsAndInstances()
	if err != nil {
		return fmt.Errorf("doWork failed to get instances from EC2: %v", err)
	}

	awsAdapter.UpdateTargetGroupsAndAutoScalingGroups(stacks)
	log.Printf("Found %d auto scaling group(s)", len(awsAdapter.AutoScalingGroupNames()))
	log.Printf("Found %d single instance(s)", len(awsAdapter.SingleInstances()))
	log.Printf("Found %d EC2 instance(s)", awsAdapter.CachedInstances())

	model := buildManagedModel(certsProvider, certsPerALB, certTTL, ingresses, stacks)
	log.Printf("Have %d model(s)", len(model))
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

func buildManagedModel(certsProvider certs.CertificatesProvider, certsPerALB int, certTTL time.Duration, ingresses []*kubernetes.Ingress, stacks []*aws.Stack) []*loadBalancer {
	sort.Slice(stacks, func(i, j int) bool {
		if len(stacks[i].CertificateARNs) == len(stacks[j].CertificateARNs) {
			return stacks[i].Name < stacks[j].Name
		}
		return len(stacks[i].CertificateARNs) > len(stacks[j].CertificateARNs)
	})

	model := make([]*loadBalancer, 0, len(stacks))
	for _, stack := range stacks {
		lb := &loadBalancer{
			stack:     stack,
			ingresses: make(map[string][]*kubernetes.Ingress),
			scheme:    stack.Scheme,
			shared:    stack.OwnerIngress == "",
			certTTL:   certTTL,
		}
		model = append(model, lb)
	}

	var (
		certificateARNs []string
		err             error
	)
	for _, ingress := range ingresses {
		certificateARN := ingress.CertificateARN
		if certificateARN != "" {
			certificateARNs = append(certificateARNs, certificateARN)
		}

		if len(certificateARN) == 0 { // do discovery
			certificateARNs, err = discoverCertificates(certsProvider, ingress)
			if err != nil {
				log.Printf("failed to find certificates for %v: %v", ingress.Hostnames, err)
				continue
			}
		} else { // validate that certificateARN exists
			err := checkCertificate(certsProvider, certificateARN)
			if err != nil {
				log.Printf("Failed to find certificates with ARN %s: %v", certificateARNs, err)
				continue
			}
		}

		// try to add ingress to existing ALB stacks until certificate
		// limit is exeeded.
		added := false
		for _, lb := range model {
			if lb.AddIngress(certificateARNs, ingress, certsPerALB) {
				added = true
				break
			}
		}

		// if the ingress was not added to the ALB stack because of
		// non-matching scheme or too many certificates, add a new
		// stack.
		if !added {
			i := make(map[string][]*kubernetes.Ingress, len(certificateARNs))
			for _, certificateARN := range certificateARNs {
				i[certificateARN] = []*kubernetes.Ingress{ingress}
			}
			model = append(model, &loadBalancer{ingresses: i, scheme: ingress.Scheme, shared: ingress.Shared})
		}
	}

	return model
}

func discoverCertificates(certsProvider certs.CertificatesProvider, ingress *kubernetes.Ingress) ([]string, error) {
	knownCertificates, err := certsProvider.GetCertificates()
	if err != nil {
		return nil, fmt.Errorf("discoverCertificateAndUpdateIngress failed to obtain certificates: %v", err)
	}

	certificateSummaries := certs.FindBestMatchingCertificates(knownCertificates, ingress.Hostnames)

	if len(certificateSummaries) < 1 {
		return nil, fmt.Errorf("Failed to find any certificates for hostnames: %s", ingress.Hostnames)
	}

	certs := make([]string, 0, len(certificateSummaries))
	for _, cert := range certificateSummaries {
		certs = append(certs, cert.ID())
	}

	return certs, nil
}

// checkCertificate checks that a certificate with the specified ARN exists in
// the account. Returns error if no certificate is found.
func checkCertificate(certsProvider certs.CertificatesProvider, arn string) error {
	certs, err := certsProvider.GetCertificates()
	if err != nil {
		return err
	}

	for _, cert := range certs {
		if arn == cert.ID() {
			return nil
		}
	}

	return fmt.Errorf("certificate not found")
}

func createStack(awsAdapter *aws.Adapter, lb *loadBalancer) {
	certificates := make([]string, 0, len(lb.ingresses))
	for cert, _ := range lb.ingresses {
		certificates = append(certificates, cert)
	}

	log.Printf("creating stack for certificates %q / ingress %q", certificates, lb.ingresses)

	stackId, err := awsAdapter.CreateStack(certificates, lb.scheme, lb.Owner())
	if err != nil {
		if isAlreadyExistsError(err) {
			lb.stack, err = awsAdapter.GetStack(stackId)
			if err == nil {
				return
			}
		}
		log.Printf("createStack(%q) failed: %v", certificates, err)
	} else {
		log.Printf("stack %q for certificates %q created", stackId, certificates)
	}
}

func updateStack(awsAdapter *aws.Adapter, lb *loadBalancer) {
	certificates := lb.CertificateARNs()

	log.Printf("updating %q stack for %d certificates / %d ingresses", lb.scheme, len(certificates), len(lb.ingresses))

	stackId, err := awsAdapter.UpdateStack(lb.stack.Name, certificates, lb.scheme)
	if isNoUpdatesToBePerformedError(err) {
		log.Printf("stack(%q) is already up to date", certificates)
	} else if err != nil {
		log.Printf("updateStack(%q) failed: %v", certificates, err)
	} else {
		log.Printf("stack %q for certificate %q updated", stackId, certificates)
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
	if lb.stack == nil {
		return
	}
	dnsName := strings.ToLower(lb.stack.DNSName) // lower case to satisfy Kubernetes reqs
	for _, ingresses := range lb.ingresses {
		for _, ing := range ingresses {
			if err := kubeAdapter.UpdateIngressLoadBalancer(ing, dnsName); err != nil {
				if err == kubernetes.ErrUpdateNotNeeded {
					log.Printf("Ingress update not needed %v with DNS name %q", ing, dnsName)
				} else {
					log.Printf("Failed to update ingress: %v", err)
				}
			} else {
				log.Printf("updated ingress %v with DNS name %q", ing, dnsName)
			}
		}
	}
}

func deleteStack(awsAdapter *aws.Adapter, lb *loadBalancer) {
	stackName := lb.stack.Name
	if err := awsAdapter.DeleteStack(lb.stack); err != nil {
		log.Printf("deleteStack failed to delete stack %q: %v", stackName, err)
	} else {
		log.Printf("deleted orphaned stack %q", stackName)
	}
}
