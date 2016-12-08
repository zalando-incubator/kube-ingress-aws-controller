package main

import (
	"log"
	"time"

	"fmt"

	"github.com/zalando-incubator/kube-ingress-aws-controller/aws"
	"github.com/zalando-incubator/kube-ingress-aws-controller/kubernetes"
)

func startPolling(awsAdapter *aws.Adapter, kubernetesClient *kubernetes.Client, pollingInterval time.Duration) {
	for {
		il, err := kubernetes.ListIngress(kubernetesClient)
		if err != nil {
			log.Println(err)
		} else {
			arns := flattenIngressByARN(il)
			if missingARNs := filterExistingARNs(awsAdapter, arns); len(missingARNs) > 0 {
				for missingARN, ingresses := range missingARNs {
					lb, err := createMissingLoadBalancer(awsAdapter, missingARN)
					if err != nil {
						log.Println(err)
						break
					}
					// TODO: attach LoadBalancer to AutoScalingGroup ?
					log.Printf("successfully created ALB %q for certificate %q\n", lb.ARN(), missingARN)

					if err := kubernetes.UpdateIngressLoaBalancer(kubernetesClient, ingresses, lb); err != nil {
						log.Println(err)
					} else {
						log.Printf("updated ingresses %v with DNS name %q\n", ingresses, lb.DNSName())
					}
				}
			}
		}

		time.Sleep(pollingInterval)
	}
}

func flattenIngressByARN(il *kubernetes.IngressList) map[string][]kubernetes.Ingress {
	uniqueARNs := make(map[string][]kubernetes.Ingress)
	for _, ingress := range il.Items {
		certificateARN := ingress.CertificateARN()
		uniqueARNs[certificateARN] = append(uniqueARNs[certificateARN], ingress)
	}
	return uniqueARNs
}

func filterExistingARNs(awsAdapter *aws.Adapter, certificateARNs map[string][]kubernetes.Ingress) map[string][]kubernetes.Ingress {
	missingARNs := make(map[string][]kubernetes.Ingress)
	for certificateARN, ingresses := range certificateARNs {
		lb, err := awsAdapter.FindLoadBalancerWithCertificateID(certificateARN)
		if err != nil {
			log.Println(err)
			continue
		}
		if lb != nil {
			log.Printf("found existing ALB %q with certificate %q\n", lb.Name(), certificateARN)
		} else {
			missingARNs[certificateARN] = ingresses
		}
	}
	return missingARNs
}

func createMissingLoadBalancer(awsAdapter *aws.Adapter, certificateARN string) (*aws.LoadBalancer, error) {
	log.Printf("creating ALB for ARN %q\n", certificateARN)
	lb, err := awsAdapter.CreateLoadBalancer(certificateARN)
	if err != nil {
		return nil, fmt.Errorf("failed to create ALB for certificate %q. %v\n", certificateARN, err)
	}
	return lb, nil
}
