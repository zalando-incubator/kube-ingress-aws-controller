package main

import (
	"log"
	"os"
	"os/signal"
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
	ingresses []*kubernetes.Ingress
	stack     *aws.Stack
}

const (
	ready int = iota
	missing
	marktodelete
	orphan
)

const (
	maxTargetGroupSupported = 1000
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
	return ready
}

func waitForTerminationSignals(signals ...os.Signal) chan os.Signal {
	c := make(chan os.Signal, 1)
	signal.Notify(c, signals...)
	return c
}

func startPolling(quitCH chan struct{}, certsProvider certs.CertificatesProvider, awsAdapter *aws.Adapter, kubeAdapter *kubernetes.Adapter, pollingInterval, updateStackInterval time.Duration) {
	items := make(chan *managedItem, maxTargetGroupSupported)
	go updateStacks(awsAdapter, updateStackInterval, items)
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

func updateStacks(awsAdapter *aws.Adapter, interval time.Duration, items <-chan *managedItem) {
	for {
		itemsMap := map[string]*managedItem{}
		done := make(chan struct{})
		go func() {
			for {
				select {
				case item := <-items:
					if _, ok := itemsMap[item.stack.CertificateARN()]; !ok {
						itemsMap[item.stack.CertificateARN()] = item
					}
				case <-done:
					return
				}
			}
		}()
		time.Sleep(interval)
		done <- struct{}{}
		for _, item := range itemsMap {
			updateStack(awsAdapter, item)
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

	nodes, err := kubeAdapter.ListNode()
	if err != nil {
		return fmt.Errorf("doWork failed to list node resources: %v", err)
	}
	log.Printf("Found %d nodes", len(nodes))

	stacks, err := awsAdapter.FindManagedStacks()
	if err != nil {
		return fmt.Errorf("doWork failed to list managed stacks: %v", err)
	}
	log.Printf("Found %d stacks", len(stacks))

	awsAdapter.UpdateAutoScalingGroups(extractIps(nodes))
	log.Printf("Found %d auto scaling groups", len(awsAdapter.AutoScalingGroupNames()))

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
			items <- managedItem
			updateIngress(kubeAdapter, managedItem)
		}
	}

	return nil
}

func buildManagedModel(certsProvider certs.CertificatesProvider, ingresses []*kubernetes.Ingress, stacks []*aws.Stack) map[string]*managedItem {
	model := make(map[string]*managedItem)
	for _, stack := range stacks {
		model[stack.CertificateARN()+"/"+stack.Scheme()] = &managedItem{stack: stack}
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
		if item, ok := model[certificateARN+"/"+ingress.Scheme()]; ok {
			item.ingresses = append(item.ingresses, ingress)
		} else {
			model[certificateARN+"/"+ingress.Scheme()] = &managedItem{ingresses: []*kubernetes.Ingress{ingress}}
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
	certificateARN := item.ingresses[0].CertificateARN()
	scheme := item.ingresses[0].Scheme()
	log.Printf("creating %q stack for certificate %q / ingress %q", scheme, certificateARN, item.ingresses)

	stackId, err := awsAdapter.CreateStack(certificateARN, scheme)
	if err != nil {
		if isAlreadyExistsError(err) {
			item.stack, err = awsAdapter.GetStack(stackId)
			if err == nil {
				return
			}
		}
		log.Printf("createStack(%q) failed: %v", certificateARN, err)
	} else {
		log.Printf("stack %q for certificate %q created", stackId, certificateARN)
	}
}

func updateStack(awsAdapter *aws.Adapter, item *managedItem) {
	certificateARN := item.ingresses[0].CertificateARN()
	scheme := item.ingresses[0].Scheme()
	log.Printf("updating %q stack for certificate %q / ingress %q", scheme, certificateARN, item.ingresses)

	stackId, err := awsAdapter.UpdateStack(item.stack.Name(), certificateARN, scheme)
	if isNoUpdatesToBePerformedError(err) {
		log.Printf("stack(%q) is already up to date", certificateARN)
	} else if err != nil {
		log.Printf("updateStack(%q) failed: %v", certificateARN, err)
	} else {
		log.Printf("stack %q for certificate %q updated", stackId, certificateARN)
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
	for _, ing := range item.ingresses {
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

// Extract internal IPs of Kubernetes nodes. Skip nodes with empty
// internal IP.
func extractIps(nodes []*kubernetes.Node) []string {
	result := make([]string, 0, len(nodes))
	for _, node := range nodes {
		ip := node.InternalIp()
		if ip != "" {
			result = append(result, ip)
		}
	}
	return result
}
