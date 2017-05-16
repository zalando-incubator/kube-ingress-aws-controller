package main

import (
	"log"
	"time"

	"fmt"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/pkg/errors"
	"github.com/zalando-incubator/kube-ingress-aws-controller/aws"
	"github.com/zalando-incubator/kube-ingress-aws-controller/certs"
	"github.com/zalando-incubator/kube-ingress-aws-controller/kubernetes"
	"runtime/debug"
	"strings"
)

type managedItem struct {
	ingress *kubernetes.Ingress
	stack   *aws.Stack
}

const (
	ready int = iota
	missing
	orphan
)

func (item *managedItem) Status() int {
	if item.stack != nil && item.ingress == nil {
		return orphan
	}
	if item.ingress != nil && item.stack == nil {
		return missing
	}
	return ready
}

func startPolling(certsProvider certs.CertificatesProvider, awsAdapter *aws.Adapter, kubeAdapter *kubernetes.Adapter, pollingInterval time.Duration) {
	for {
		if err := doWork(certsProvider, awsAdapter, kubeAdapter); err != nil {
			log.Println(err)
		}
		time.Sleep(pollingInterval)
	}
}

func doWork(certsProvider certs.CertificatesProvider, awsAdapter *aws.Adapter, kubeAdapter *kubernetes.Adapter) error {
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

	stacks, err := awsAdapter.FindManagedStacks()
	if err != nil {
		return fmt.Errorf("doWork failed to list managed stacks: %v", err)
	}

	model := buildManagedModel(certsProvider, ingresses, stacks)
	for _, managedItem := range model {
		switch managedItem.Status() {
		case orphan:
			deleteStack(awsAdapter, managedItem)
		case missing:
			createStack(awsAdapter, managedItem)
			fallthrough
		case ready:
			updateIngress(kubeAdapter, managedItem)
		}
	}

	return nil
}

func buildManagedModel(certsProvider certs.CertificatesProvider, ingresses []*kubernetes.Ingress, stacks []*aws.Stack) map[string]*managedItem {
	model := make(map[string]*managedItem)
	for _, stack := range stacks {
		model[stack.CertificateARN()] = &managedItem{stack: stack}
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
				log.Printf("failed to find a certificate for %v: %v\n", ingress.CertHostname(), err)
				continue
			}
		}
		if item, ok := model[certificateARN]; ok {
			item.ingress = ingress
		} else {
			model[certificateARN] = &managedItem{ingress: ingress}
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

func createStack(awsAdapter *aws.Adapter, item *managedItem) {
	certificateARN := item.ingress.CertificateARN()
	log.Printf("creating stack for certificate %q / ingress %q\n", certificateARN, item.ingress)

	stackId, err := awsAdapter.CreateStack(certificateARN)
	if err != nil {
		if isAlreadyExistsError(err) {
			item.stack, err = awsAdapter.GetStack(stackId)
			if err == nil {
				return
			}
		}
		log.Printf("createStack(%q) failed: %v\n", certificateARN, err)
	} else {
		log.Printf("stack %q for certificate %q created\n", stackId, certificateARN)
	}
}

func isAlreadyExistsError(err error) bool {
	if awsErr, ok := err.(awserr.Error); ok {
		return awsErr.Code() == cloudformation.ErrCodeAlreadyExistsException
	}
	return false
}

func updateIngress(kubeAdapter *kubernetes.Adapter, item *managedItem) {
	if item.stack == nil {
		return
	}
	dnsName := strings.ToLower(item.stack.DNSName()) // lower case to satisfy Kubernetes reqs
	if err := kubeAdapter.UpdateIngressLoadBalancer(item.ingress, dnsName); err != nil {
		if err != kubernetes.ErrUpdateNotNeeded {
			log.Println(err)
		}
	} else {
		log.Printf("updated ingress %v with DNS name %q\n", item.ingress, dnsName)
	}
}

func deleteStack(awsAdapter *aws.Adapter, item *managedItem) {
	stackName := item.stack.Name()
	if err := awsAdapter.DeleteStack(item.stack); err != nil {
		log.Printf("deleteStack failed to delete stack %q: %v\n", stackName, err)
	} else {
		log.Printf("deleted orphaned stack %q\n", stackName)
	}
}
