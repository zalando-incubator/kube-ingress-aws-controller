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

type managedItem struct {
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
	maxCertsPerALBSupported = 25
)

func (item *managedItem) Status() int {
	if item.stack.ShouldDelete() {
		return delete
	}
	if len(item.ingresses) != 0 && item.stack == nil {
		return missing
	}
	if !item.certsEqual() && item.stack.IsComplete() {
		return update
	}
	return ready
}

// certsEqual checks if the certs found for the ingresses match those already
// defined on the LB stack.
func (item *managedItem) certsEqual() bool {
	if len(item.ingresses) != len(item.stack.CertificateARNs()) {
		return false
	}

	for arn, _ := range item.ingresses {
		if ttl, ok := item.stack.CertificateARNs()[arn]; !ok || !ttl.IsZero() {
			return false
		}
	}
	return true
}

// AddIngress adds an ingress object to the managed item.
// The function returns true when the ingress was successfully added. The
// adding can fail in case the managed item reached its limit of ingress
// certificates (25 max) or if the scheme doesn't match.
func (item *managedItem) AddIngress(certificateARNs []string, ingress *kubernetes.Ingress, maxCerts int) bool {
	if item.scheme != ingress.Scheme() {
		return false
	}

	resourceName := fmt.Sprintf("%s/%s", ingress.Namespace(), ingress.Name())

	owner := item.stack.OwnerIngress()

	if !ingress.Shared() && resourceName != owner {
		return false
	}

	for _, certificateARN := range certificateARNs {
		if ingresses, ok := item.ingresses[certificateARN]; ok {
			item.ingresses[certificateARN] = append(ingresses, ingress)
		} else {
			if len(item.ingresses) >= maxCerts {
				return false
			}
			item.ingresses[certificateARN] = []*kubernetes.Ingress{ingress}
		}
	}

	item.shared = ingress.Shared()

	return true
}

// CertificateARNs returns a map of certificates and their expiry times.
func (item *managedItem) CertificateARNs() map[string]time.Time {
	certificates := make(map[string]time.Time, len(item.ingresses))
	for arn := range item.ingresses {
		certificates[arn] = time.Time{}
	}

	for arn, ttl := range item.stack.CertificateARNs() {
		if _, ok := certificates[arn]; !ok {
			if ttl.IsZero() {
				certificates[arn] = time.Now().UTC().Add(item.certTTL)
			} else if ttl.After(time.Now().UTC()) {
				certificates[arn] = ttl
			}
		}
	}

	return certificates
}

// Owner returns the ingress resource owning the item. If there are no owners
// it will return an empty string meaning the item is shared between multiple
// ingresses.
func (item *managedItem) Owner() string {
	if item.shared {
		return ""
	}

	for _, ingresses := range item.ingresses {
		for _, ingress := range ingresses {
			return fmt.Sprintf("%s/%s", ingress.Namespace(), ingress.Name())
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
	items := make(chan *managedItem, maxTargetGroupSupported)
	for {
		log.Printf("Start polling sleep %s", pollingInterval)
		select {
		case <-waitForTerminationSignals(syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT):
			quitCH <- struct{}{}
			return
		case <-time.After(pollingInterval):
			if err := doWork(certsProvider, certsPerALB, certTTL, awsAdapter, kubeAdapter, items); err != nil {
				log.Println(err)
			}
		}
	}
}

func doWork(certsProvider certs.CertificatesProvider, certsPerALB int, certTTL time.Duration, awsAdapter *aws.Adapter, kubeAdapter *kubernetes.Adapter, items chan<- *managedItem) error {
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
	log.Printf("Found %d ingresses", len(ingresses))

	stacks, err := awsAdapter.FindManagedStacks()
	if err != nil {
		return fmt.Errorf("doWork failed to list managed stacks: %v", err)
	}
	log.Printf("Found %d stacks", len(stacks))

	err = awsAdapter.UpdateAutoScalingGroupsAndInstances()
	if err != nil {
		return fmt.Errorf("doWork failed to get instances from EC2: %v", err)
	}

	awsAdapter.UpdateTargetGroupsAndAutoScalingGroups(stacks)
	log.Printf("Found %d auto scaling groups", len(awsAdapter.AutoScalingGroupNames()))
	log.Printf("Found %d single instances", len(awsAdapter.SingleInstances()))
	log.Printf("Found %d EC2 instances", awsAdapter.CachedInstances())

	model := buildManagedModel(certsProvider, certsPerALB, certTTL, ingresses, stacks)
	log.Printf("Have %d models", len(model))
	for _, managedItem := range model {
		switch managedItem.Status() {
		case delete:
			deleteStack(awsAdapter, managedItem)
		case missing:
			createStack(awsAdapter, managedItem)
			updateIngress(kubeAdapter, managedItem)
		case ready:
			updateIngress(kubeAdapter, managedItem)
		case update:
			updateStack(awsAdapter, managedItem)
			updateIngress(kubeAdapter, managedItem)
		}
	}

	return nil
}

func buildManagedModel(certsProvider certs.CertificatesProvider, certsPerALB int, certTTL time.Duration, ingresses []*kubernetes.Ingress, stacks []*aws.Stack) []*managedItem {
	sort.Slice(stacks, func(i, j int) bool {
		if len(stacks[i].CertificateARNs()) == len(stacks[j].CertificateARNs()) {
			return stacks[i].Name() < stacks[j].Name()
		}
		return len(stacks[i].CertificateARNs()) > len(stacks[j].CertificateARNs())
	})

	model := make([]*managedItem, 0, len(stacks))
	for _, stack := range stacks {
		item := &managedItem{
			stack:     stack,
			ingresses: make(map[string][]*kubernetes.Ingress),
			scheme:    stack.Scheme(),
			shared:    stack.OwnerIngress() == "",
			certTTL:   certTTL,
		}
		model = append(model, item)
	}

	var (
		certificateARNs []string
		err             error
	)
	for _, ingress := range ingresses {
		certificateARN := ingress.CertificateARN()
		if certificateARN != "" {
			certificateARNs = append(certificateARNs, certificateARN)
		}

		if len(certificateARN) == 0 { // do discovery
			certificateARNs, err = discoverCertificates(certsProvider, ingress)
			if err != nil {
				log.Printf("failed to find certificates for %v: %v", ingress.CertHostnames(), err)
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
		for _, item := range model {
			if item.AddIngress(certificateARNs, ingress, certsPerALB) {
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
			model = append(model, &managedItem{ingresses: i, scheme: ingress.Scheme(), shared: ingress.Shared()})
		}
	}

	return model
}

func discoverCertificates(certsProvider certs.CertificatesProvider, ingress *kubernetes.Ingress) ([]string, error) {
	knownCertificates, err := certsProvider.GetCertificates()
	if err != nil {
		return nil, fmt.Errorf("discoverCertificateAndUpdateIngress failed to obtain certificates: %v", err)
	}

	certificateSummaries, err := certs.FindBestMatchingCertificates(knownCertificates, ingress.CertHostnames())
	if err != nil {
		return nil, fmt.Errorf("discoverCertificateAndUpdateIngress failed to find a certificate for %q: %v",
			ingress.CertHostnames(), err)
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

func createStack(awsAdapter *aws.Adapter, item *managedItem) {
	certificates := make([]string, 0, len(item.ingresses))
	for cert, _ := range item.ingresses {
		certificates = append(certificates, cert)
	}

	log.Printf("creating stack for certificates %q / ingress %q", certificates, item.ingresses)

	stackId, err := awsAdapter.CreateStack(certificates, item.scheme, item.Owner())
	if err != nil {
		if isAlreadyExistsError(err) {
			item.stack, err = awsAdapter.GetStack(stackId)
			if err == nil {
				return
			}
		}
		log.Printf("createStack(%q) failed: %v", certificates, err)
	} else {
		log.Printf("stack %q for certificates %q created", stackId, certificates)
	}
}

func updateStack(awsAdapter *aws.Adapter, item *managedItem) {
	certificates := item.CertificateARNs()

	log.Printf("updating %q stack for %d certificates / %d ingresses", item.scheme, len(certificates), len(item.ingresses))

	stackId, err := awsAdapter.UpdateStack(item.stack.Name(), certificates, item.scheme)
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

func updateIngress(kubeAdapter *kubernetes.Adapter, item *managedItem) {
	if item.stack == nil {
		return
	}
	dnsName := strings.ToLower(item.stack.DNSName()) // lower case to satisfy Kubernetes reqs
	for _, ingresses := range item.ingresses {
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

func deleteStack(awsAdapter *aws.Adapter, item *managedItem) {
	stackName := item.stack.Name()
	if err := awsAdapter.DeleteStack(item.stack); err != nil {
		log.Printf("deleteStack failed to delete stack %q: %v", stackName, err)
	} else {
		log.Printf("deleted orphaned stack %q", stackName)
	}
}
