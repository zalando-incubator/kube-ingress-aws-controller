package main

import (
	"log"
	"time"

	"fmt"

	"github.com/pkg/errors"
	"github.com/zalando-incubator/kube-ingress-aws-controller/aws"
	"github.com/zalando-incubator/kube-ingress-aws-controller/kubernetes"
)

func startPolling(awsAdapter *aws.Adapter, kubeAdapter *kubernetes.Adapter, pollingInterval time.Duration) {
	for {
		if err := doWork(awsAdapter, kubeAdapter); err != nil {
			log.Println(err)
		}
		time.Sleep(pollingInterval)
	}
}

func doWork(awsAdapter *aws.Adapter, kubeAdapter *kubernetes.Adapter) error {
	defer func() error {
		if r := recover(); r != nil {
			log.Println("shit has hit the fan:", errors.Wrap(r.(error), "panic caused by"))
			return r.(error)
		}
		return nil
	}()

	ingresses, err := kubeAdapter.ListIngress()
	if err != nil {
		return err
	}
	log.Printf("found %d ingress resource(s)", len(ingresses))

	acmCerts := awsAdapter.GetCerts()
	log.Printf("found %d ACM certificates", len(acmCerts))

	uniqueARNs := ingressByCertArn(awsAdapter, ingresses, acmCerts)
	missingARNs, existingARNs := filterExistingARNs(awsAdapter, uniqueARNs)
	for missingARN, ingresses := range missingARNs {
		lb, err := createMissingLoadBalancer(awsAdapter, missingARN)
		if err != nil {
			log.Println(err)
			continue
		}
		log.Printf("successfully created ALB %q for certificate %q\n", lb.ARN(), missingARN)

		updateIngresses(kubeAdapter, ingresses, lb.DNSName())
	}

	for existingARN, lb := range existingARNs {
		ingresses := uniqueARNs[existingARN]
		updateIngresses(kubeAdapter, ingresses, lb.DNSName())
	}

	if err := deleteOrphanedLoadBalancers(awsAdapter, ingresses); err != nil {
		log.Println("failed to delete orphaned load balancers", err)
	}

	return nil
}

func ingressByCertArn(awsAdapter *aws.Adapter, ingresses []*kubernetes.Ingress, acmCerts []*aws.CertDetail) map[string][]*kubernetes.Ingress {
	uniqueARNs := make(map[string][]*kubernetes.Ingress)
	for _, ingress := range ingresses {
		certificateARN := ingress.CertificateARN()
		if certificateARN != "" {
			uniqueARNs[certificateARN] = append(uniqueARNs[certificateARN], ingress)
		} else {
			arn, err := findCertARNForIngress(awsAdapter, ingress, acmCerts)
			if err != nil {
				log.Printf("No valid Certificate found for %v: %v", ingress.CertHostname(), err)
				continue
			}
			log.Printf("using autopilot mode for ingress %v", ingress)
			uniqueARNs[arn] = append(uniqueARNs[arn], ingress)
		}
	}
	return uniqueARNs
}

func findCertARNForIngress(awsAdapter *aws.Adapter, ingress *kubernetes.Ingress, acmCerts []*aws.CertDetail) (string, error) {
	acmCert, err := awsAdapter.FindBestMatchingCertificate(acmCerts, ingress.CertHostname())
	if err != nil {
		return "", err
	}
	ingress.SetCertificateARN(acmCert.Arn)

	return acmCert.Arn, nil
}

func filterExistingARNs(awsAdapter *aws.Adapter, certificateARNs map[string][]*kubernetes.Ingress) (map[string][]*kubernetes.Ingress, map[string]*aws.LoadBalancer) {
	missingARNs := make(map[string][]*kubernetes.Ingress)
	existingARNs := make(map[string]*aws.LoadBalancer)
	for certificateARN, ingresses := range certificateARNs {
		lb, err := awsAdapter.FindLoadBalancerWithCertificateID(certificateARN)
		if err != nil && err != aws.ErrLoadBalancerNotFound {
			log.Println(err)
			continue
		}
		if lb != nil {
			log.Printf("found existing ALB %q with certificate %q\n", lb.Name(), certificateARN)
			existingARNs[certificateARN] = lb
		} else {
			log.Printf("ALB with certificate %q not found\n", certificateARN)
			missingARNs[certificateARN] = ingresses
		}
	}
	return missingARNs, existingARNs
}

func createMissingLoadBalancer(awsAdapter *aws.Adapter, certificateARN string) (*aws.LoadBalancer, error) {
	log.Printf("creating ALB for ARN %q\n", certificateARN)

	lb, err := awsAdapter.CreateLoadBalancer(certificateARN)
	if err != nil {
		return nil, fmt.Errorf("failed to create ALB for certificate %q. %v\n", certificateARN, err)
	}
	return lb, nil
}

func updateIngresses(kubeAdapter *kubernetes.Adapter, ingresses []*kubernetes.Ingress, dnsName string) {
	for _, ingress := range ingresses {
		if err := kubeAdapter.UpdateIngressLoadBalancer(ingress, dnsName); err != nil {
			log.Println(err)
		} else {
			log.Printf("updated ingress %v with DNS name %q\n", ingress, dnsName)
		}
	}
}

func deleteOrphanedLoadBalancers(awsAdapter *aws.Adapter, ingresses []*kubernetes.Ingress) error {
	lbs, err := awsAdapter.FindManagedLoadBalancers()
	if err != nil {
		return err
	}

	certificateMap := make(map[string]bool)
	for _, ingress := range ingresses {
		certificateMap[ingress.CertificateARN()] = true
	}

	for _, lb := range lbs {
		if _, has := certificateMap[lb.CertificateARN()]; !has {
			if err := awsAdapter.DeleteLoadBalancer(lb); err == nil {
				log.Printf("deleted orphaned load balancer ARN %q\n", lb.ARN())
			} else {
				log.Printf("failed to delete orphaned load balancer ARN %q: %v\n", lb.ARN(), err)
			}
		}
	}
	return nil
}
