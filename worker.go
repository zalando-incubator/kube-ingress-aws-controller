package main

import (
	"errors"
	"log"
	"strings"
	"time"

	"fmt"

	awssdk "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/acm"
	"github.com/zalando-incubator/kube-ingress-aws-controller/aws"
	"github.com/zalando-incubator/kube-ingress-aws-controller/kubernetes"
)

const (
	// GLOB is used in Glob() and corresponds to the X509 CN/AlternateName wildcard char
	GLOB = "*"
)

var (
	// ErrNoMatchingCertificateFound is used if there is no matching ACM certificate found
	ErrNoMatchingCertificateFound = errors.New("skipper-ingress-controller: no matching ACM certificate found")
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
			log.Println("shit has hit the fan:", r)
			return r.(error)
		}
		return nil
	}()

	ingresses, err := kubeAdapter.ListIngress() // default if not set: Ingress.certificateARN = ""
	if err != nil {
		return err
	}
	log.Printf("found %d ingress resource(s)", len(ingresses))
	log.Printf("TRACE: ingress resource(s): %+v", ingresses)

	acmCerts := awsAdapter.GetCachedCerts()

	log.Printf("found %d ACM certificates", len(acmCerts))
	log.Printf("TRACE: CERTS: %+v", acmCerts) // TODO(sszuecss): drop this
	// TODO(sszuecs): ACM certificates can not be safely
	// updated. We have to get CertificateDetail
	// https://github.com/aws/aws-sdk-go/blob/master/service/acm/api.go#L947-L1040
	// in order to get NotAfter, NotBefore and RevokedAt time.Time
	// for a certificate and filter out older certificates with
	// the same DomainName and SubjectAlternativeNames.

	uniqueARNs := flattenIngressByARN(ingresses)
	log.Printf("TRACE1: %d uniqueARNs: %+v", len(uniqueARNs), uniqueARNs)
	// all ingresses with Ingress.certificateARN == "" should get lookuped their cert
	setCertARNsForIngress(uniqueARNs[""], acmCerts)
	newFlattenedArns := flattenIngressByARN(uniqueARNs[""])
	// delete all Ingress that we did not found a cert for
	if ingressesWithoutCert, ok := newFlattenedArns[""]; ok {
		log.Printf("TRACE: found empty string in newFlattenedArns")
		for _, i := range ingressesWithoutCert {
			log.Printf("No matching ACM certificate found ingress: %s and hostname: %s", i, i.CertHostname())
		}
		delete(newFlattenedArns, "")
	}
	if _, ok := newFlattenedArns[""]; ok {
		log.Printf("TRACE: empty string in newFlattenedArns should be deleted")
	}

	log.Printf("found %d ingress resource(s), that had not a certificate ARN", len(ingresses))
	log.Printf("TRACE: ingress resource(s) after setCertARNsForIngress: %+v", ingresses)
	for _, ingress := range ingresses {
		err := kubeAdapter.UpdateIngressARN(ingress)
		if err != nil {
			log.Printf("Could not update Kubernetes ingress to set CertARN annotation: %v", err)
		}
	}

	log.Printf("TRACE2: %d uniqueARNs: %+v", len(uniqueARNs), uniqueARNs)

	// merge with uniquARNs
	for k, v := range newFlattenedArns {
		if _, ok := uniqueARNs[k]; !ok {
			// case new key -> create
			uniqueARNs[k] = make([]*kubernetes.Ingress, 0, 1)
		}
		uniqueARNs[k] = append(uniqueARNs[k], v...)
	}
	delete(uniqueARNs, "")
	log.Printf("TRACE3: %d uniqueARNs: %+v", len(uniqueARNs), uniqueARNs)

	// TODO: does not work as I expected
	missingARNs, existingARNs := filterExistingARNs(awsAdapter, uniqueARNs) // create ingress
	for missingARN, ingresses := range missingARNs {
		log.Printf("TRACE: createMissingLoadBalancer missingARNs: %+v, ingresses: %+v", missingARN, ingresses)
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

func setCertARNsForIngress(ingresses []*kubernetes.Ingress, certs []*acm.CertificateDetail) error {
	for _, ing := range ingresses {
		acmCert, err := FindBestMatchingCertifcate(certs, ing.CertHostname())
		if err != nil {
			log.Printf("No matching Certificate found for hostname: %s", ing.CertHostname())
			continue
		}
		// TODO(sszuecs): drop %+v
		log.Printf("Found best matching cert for %s: %+v, ARN: %s", ing.CertHostname(), acmCert, awssdk.StringValue(acmCert.CertificateArn))
		ing.SetCertificateARN(awssdk.StringValue(acmCert.CertificateArn))
	}

	return nil
}

// assumption
//     1 ALB --- 1 cert ---< n ingress
// response {certARN1: []Ingress, ..}
func flattenIngressByARN(ingresses []*kubernetes.Ingress) map[string][]*kubernetes.Ingress {
	uniqueARNs := make(map[string][]*kubernetes.Ingress)
	for _, ingress := range ingresses {
		certificateARN := ingress.CertificateARN()
		uniqueARNs[certificateARN] = append(uniqueARNs[certificateARN], ingress)
	}
	return uniqueARNs
}

func filterExistingARNs(awsAdapter *aws.Adapter, certificateARNs map[string][]*kubernetes.Ingress) (map[string][]*kubernetes.Ingress, map[string]*aws.LoadBalancer) {
	missingARNs := make(map[string][]*kubernetes.Ingress)
	existingARNs := make(map[string]*aws.LoadBalancer)
	for certificateARN, ingresses := range certificateARNs {
		log.Printf("TRACE: filterExistingARNs certARN: %+v", certificateARN)
		lb, err := awsAdapter.FindLoadBalancerWithCertificateID(certificateARN)
		log.Printf("TRACE: awsAdapter.FindLoadBalancerWithCertificateID returned: %v/%v", lb, err)
		if err != nil && err != aws.ErrLoadBalancerNotFound {
			log.Println("ERR:", err)
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
		return nil, fmt.Errorf("failed to create ALB for certificate %q: %v", certificateARN, err)
	}
	return lb, nil
}

// updateIngresses updates the status part in the Kubernetes ingress object
func updateIngresses(kubeAdapter *kubernetes.Adapter, ingresses []*kubernetes.Ingress, dnsName string) {
	for _, ingress := range ingresses {
		if err := kubeAdapter.UpdateIngressLoadBalancer(ingress, dnsName); err != nil {
			log.Println("failed to update ingress LB:", err)
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

// FindBestMatchingCertifcate get all ACM certifcates and use a suffix
// search best match opertion in order to find the best matching
// certifcate ARN.
func FindBestMatchingCertifcate(certs []*acm.CertificateDetail, hostname string) (*acm.CertificateDetail, error) {
	candidates := map[int]*acm.CertificateDetail{}
	longestGlob := -1

	for _, cert := range certs {
		if Glob(awssdk.StringValue(cert.DomainName), hostname) {
			l := len(awssdk.StringValue(cert.DomainName))
			if longestGlob < l {
				longestGlob = l
				candidates[l] = cert
			} else if longestGlob == l {
				// TODO: check cert details https://github.bus.zalan.do/teapot/issues/issues/315

			}
		}
	}

	if longestGlob == -1 {
		return nil, ErrNoMatchingCertificateFound
	}
	return candidates[longestGlob], nil
}

// modified version of https://github.com/ryanuber/go-glob/blob/master/glob.go (MIT licensed)
func Glob(pattern, subj string) bool {
	// Empty pattern can only match empty subject
	if pattern == "" {
		return subj == pattern
	}

	// If the pattern _is_ a glob, it matches everything
	if pattern == GLOB {
		return true
	}

	parts := strings.Split(pattern, GLOB)

	if len(parts) == 1 {
		// No globs in pattern, so test for equality
		return subj == pattern
	}

	leadingGlob := strings.HasPrefix(pattern, GLOB)
	end := len(parts) - 1

	// Go over the leading parts and ensure they match.
	for i := 0; i < end; i++ {
		idx := strings.Index(subj, parts[i])

		switch i {
		case 0:
			// Check the first section. Requires special handling.
			if !leadingGlob && idx != 0 {
				return false
			}
		default:
			// Check that the middle parts match.
			if idx < 0 {
				return false
			}
		}

		// Trim evaluated text from subj as we loop over the pattern.
		subj = subj[idx+len(parts[i]):]
	}

	// Reached the last section. Requires special handling.
	return strings.HasSuffix(subj, parts[end])
}
