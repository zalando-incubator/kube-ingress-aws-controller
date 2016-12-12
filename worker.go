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
		if err := doWork(awsAdapter, kubernetesClient); err != nil {
			log.Println(err)
		}
		time.Sleep(pollingInterval)
	}
}

func doWork(awsAdapter *aws.Adapter, kubernetesClient *kubernetes.Client) error {
	defer func() error {
		if r := recover(); r != nil {
			log.Println("shit has hit the fan:", r)
			return r.(error)
		}
		return nil
	}()

	il, err := kubernetes.ListIngress(kubernetesClient)
	if err != nil {
		return err
	}

	log.Printf("found %d ingress resource(s)", len(il.Items))

	uniqueARNs := flattenIngressByARN(il)
	missingARNs := filterExistingARNs(awsAdapter, uniqueARNs)
	for missingARN, ingresses := range missingARNs {
		lb, err := createMissingLoadBalancer(awsAdapter, missingARN)
		if err != nil {
			log.Println(err)
			continue
		}
		// TODO: attach LoadBalancer to AutoScalingGroup ?
		log.Printf("successfully created ALB %q for certificate %q\n", lb.ARN(), missingARN)

		if errors := kubernetes.UpdateIngressLoaBalancer(kubernetesClient, ingresses, lb.DNSName()); len(errors) > 0 {
			if len(errors) == 1 {
				log.Println(errors[0])
			} else {
				log.Println("Multiple errors occurred updating the Ingress resources:")
				for _, err := range errors {
					log.Printf("\t%v\n", err)
				}
			}
		} else {
			log.Printf("updated ingresses %v with DNS name %q\n", ingresses, lb.DNSName())
		}
	}

	log.Println("checking for orphaned load balancers")
	if err := deleteOrphanedLoadBalancers(awsAdapter, il.Items); err != nil {
		log.Println("failed to delete orphaned load balancers", err)
	}

	return nil
}

func flattenIngressByARN(il *kubernetes.IngressList) map[string][]kubernetes.Ingress {
	uniqueARNs := make(map[string][]kubernetes.Ingress)
	for _, ingress := range il.Items {
		certificateARN := ingress.CertificateARN()
		if certificateARN != "" {
			uniqueARNs[certificateARN] = append(uniqueARNs[certificateARN], ingress)
		} else {
			log.Printf("invalid/empty ARN for ingress %v\n", ingress)
		}
	}
	return uniqueARNs
}

func filterExistingARNs(awsAdapter *aws.Adapter, certificateARNs map[string][]kubernetes.Ingress) map[string][]kubernetes.Ingress {
	missingARNs := make(map[string][]kubernetes.Ingress)
	for certificateARN, ingresses := range certificateARNs {
		lb, err := awsAdapter.FindLoadBalancerWithCertificateID(certificateARN)
		if err != nil && err != aws.ErrLoadBalancerNotFound {
			log.Println(err)
			continue
		}
		if lb != nil {
			log.Printf("found existing ALB %q with certificate %q\n", lb.Name(), certificateARN)
		} else {
			log.Printf("ALB with certificate %q not found\n", certificateARN)
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

func deleteOrphanedLoadBalancers(awsAdapter *aws.Adapter, ingresses []kubernetes.Ingress) error {
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
