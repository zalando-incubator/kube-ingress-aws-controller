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

const (
	defaultCertARNTTL = 5 * time.Minute
)

type managedItem struct {
	ingresses map[string][]*kubernetes.Ingress
	scheme    string
	stack     *aws.Stack
}

const (
	ready int = iota
	update
	missing
	marktodelete
	orphan
)

const (
	maxTargetGroupSupported = 1000
	maxCertsPerALBSupported = 25
)

func (item *managedItem) Status() int {
	if item.stack.ShouldDelete() {
		return orphan
	}
	if item.stack != nil && len(item.ingresses) == 0 && !item.stack.IsDeleteInProgress() {
		return marktodelete
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

func waitForTerminationSignals(signals ...os.Signal) chan os.Signal {
	c := make(chan os.Signal, 1)
	signal.Notify(c, signals...)
	return c
}

func startPolling(quitCH chan struct{}, certsProvider certs.CertificatesProvider, awsAdapter *aws.Adapter, kubeAdapter *kubernetes.Adapter, pollingInterval time.Duration) {
	items := make(chan *managedItem, maxTargetGroupSupported)
	for {
		log.Printf("Start polling sleep %s", pollingInterval)
		select {
		case <-waitForTerminationSignals(syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT):
			quitCH <- struct{}{}
			return
		case <-time.After(pollingInterval):
			if err := doWork(certsProvider, awsAdapter, kubeAdapter, items); err != nil {
				log.Println(err)
			}
		}
	}
}

func doWork(certsProvider certs.CertificatesProvider, awsAdapter *aws.Adapter, kubeAdapter *kubernetes.Adapter, items chan<- *managedItem) error {
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

	model := buildManagedModel(certsProvider, ingresses, stacks)
	log.Printf("Have %d models", len(model))
	for _, managedItem := range model {
		switch managedItem.Status() {
		case orphan:
			deleteStack(awsAdapter, managedItem)
		case marktodelete:
			markToDeleteStack(awsAdapter, managedItem)
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

func buildManagedModel(certsProvider certs.CertificatesProvider, ingresses []*kubernetes.Ingress, stacks []*aws.Stack) []*managedItem {
	sort.Slice(stacks, func(i, j int) bool {
		return len(stacks[i].CertificateARNs()) > len(stacks[j].CertificateARNs())
	})
	model := make([]*managedItem, 0, len(stacks))
	for _, stack := range stacks {
		item := &managedItem{
			stack:     stack,
			ingresses: make(map[string][]*kubernetes.Ingress),
			scheme:    stack.Scheme(),
		}
		model = append(model, item)
	}

	var (
		certificateARN string
		err            error
	)
	for _, ingress := range ingresses {
		certificateARN = ingress.CertificateARN()
		if certificateARN == "" { // do discovery
			certificateARN, err = discoverCertificateAndUpdateIngress(certsProvider, ingress)
			if err != nil {
				log.Printf("failed to find a certificate for %v: %v", ingress.CertHostname(), err)
				continue
			}
		} else { // validate that certificateARN exists
			err := checkCertificate(certsProvider, certificateARN)
			if err != nil {
				log.Printf("Failed to find certificate with ARN %s: %v", certificateARN, err)
				continue
			}
		}

		// try to add ingress to existing ALB stacks until certificate
		// limit is exeeded.
		added := false
		for _, m := range model {
			if m.scheme != ingress.Scheme() {
				continue
			}

			if ingresses, ok := m.ingresses[certificateARN]; ok {
				m.ingresses[certificateARN] = append(ingresses, ingress)
			} else {
				if len(m.ingresses) >= maxCertsPerALBSupported {
					continue
				}
				m.ingresses[certificateARN] = []*kubernetes.Ingress{ingress}
			}
			added = true
			break
		}

		// if there were no existing ALB stack with room for one more
		// certificate, add a new one.
		if !added {
			i := map[string][]*kubernetes.Ingress{
				certificateARN: []*kubernetes.Ingress{ingress},
			}
			model = append(model, &managedItem{ingresses: i, scheme: ingress.Scheme()})
		}
	}

	return model
}

func discoverCertificateAndUpdateIngress(certsProvider certs.CertificatesProvider, ingress *kubernetes.Ingress) (string, error) {
	knownCertificates, err := certsProvider.GetCertificates()
	if err != nil {
		return "", fmt.Errorf("discoverCertificateAndUpdateIngress failed to obtain certificates: %v", err)
	}

	certificateSummary, err := certs.FindBestMatchingCertificate(knownCertificates, ingress.CertHostname())
	if err != nil {
		return "", fmt.Errorf("discoverCertificateAndUpdateIngress failed to find a certificate for %q: %v",
			ingress.CertHostname(), err)
	}
	ingress.SetCertificateARN(certificateSummary.ID())
	return certificateSummary.ID(), nil
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

	stackId, err := awsAdapter.CreateStack(certificates, item.scheme)
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
	certificates := make(map[string]time.Time, len(item.ingresses))
	for arn, ttl := range item.stack.CertificateARNs() {
		if _, ok := item.ingresses[arn]; !ok {
			if ttl.IsZero() {
				certificates[arn] = time.Now().UTC().Add(defaultCertARNTTL)
			} else if ttl.Before(time.Now().UTC()) {
				certificates[arn] = ttl
			}
			continue
		}

		certificates[arn] = time.Time{}
	}

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
				if err != kubernetes.ErrUpdateNotNeeded {
					log.Println(err)
				} else {
					log.Printf("updated ingress not needed %v with DNS name %q", ing, dnsName)
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

func markToDeleteStack(awsAdapter *aws.Adapter, item *managedItem) {
	stackName := item.stack.Name()
	ts, err := awsAdapter.MarkToDeleteStack(item.stack)
	if err != nil {
		log.Printf("markToDeleteStack failed to tag stack %q, at %v: %v", stackName, ts, err)
	} else {
		log.Printf("marked stack %q to be deleted at %v", stackName, ts)
	}
}
