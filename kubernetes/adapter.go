package kubernetes

import (
	"errors"
	"fmt"
	"log"
)

type Adapter struct {
	kubeClient client
}

var (
	// ErrMissingKubernetesEnv is returned when the Kubernetes API server environment variables are not defined
	ErrMissingKubernetesEnv = errors.New("unable to load in-cluster configuration, KUBERNETES_SERVICE_HOST and " +
		"KUBERNETES_SERVICE_PORT are not defined")
	// ErrInvalidIngressUpdateParams is returned when a request to update ingress resources has an empty DNS name
	// or doesn't specify any ingress resources
	ErrInvalidIngressUpdateParams = errors.New("invalid ingress update parameters")
	// ErrInvalidIngressUpdateARNParams is returned when a request to update ingress resources has an empty ARN
	// or doesn't specify any ingress resources
	ErrInvalidIngressUpdateARNParams = errors.New("invalid ingress updateARN parameters")
	// ErrUpdateNotNeeded is returned when an ingress update call doesn't require an update due to already having
	// the desired hostname
	ErrUpdateNotNeeded = errors.New("update to ingress resource not needed")
	// ErrInvalidConfiguration is returned when the Kubernetes configuration is missing required attributes
	ErrInvalidConfiguration = errors.New("invalid Kubernetes Adapter configuration")
	// ErrInvalidCertificate is returned when the CA certificates required to communicate with the
	// API server are invalid
	ErrInvalidCertificates = errors.New("invalid CA certificates")
)

// Ingress is the ingress-controller's business object
type Ingress struct {
	certificateARN string
	namespace      string
	name           string
	hostName       string // limitation: 1 Ingress --- 1 hostName
	ruleHostname   string
}

// CertificateARN returns the AWS certificate (IAM or ACM) ARN found in the ingress resource metadata.
// It returns an empty string if the annotation is missing.
func (i *Ingress) CertificateARN() string {
	return i.certificateARN
}

// String returns a string representation of the Ingress instance containing the namespace and the resource name.
func (i *Ingress) String() string {
	return fmt.Sprintf("%s/%s -> %s", i.namespace, i.name, i.ruleHostname)
}

// Hostname returns the DNS LoadBalancer hostname associated with the
// ingress got from Kubernetes Status
func (i *Ingress) Hostname() string {
	return i.hostName
}

// CertHostname returns the DNS hostname associated with the ingress
// got from Kubernetes Spec
func (i *Ingress) CertHostname() string {
	return i.ruleHostname
}

// SetCertificateARN sets Ingress.certificateARN to the arn as specified.
func (i *Ingress) SetCertificateARN(arn string) {
	i.certificateARN = arn
}

// newIngressFromKube has the kubernetes ingress as input parameter
// and finds the first matching LB hostname for it and creates an
// Ingress object as response.
//
// TODO: fix limitation
//
// limitation:
//
//    1 ingress --- n Host
//    1 Ingress --- 1 hostName
func newIngressFromKube(kubeIngress *ingress) *Ingress {
	var host, certHostname string
	for _, ingressLoadBalancer := range kubeIngress.Status.LoadBalancer.Ingress {
		if ingressLoadBalancer.Hostname != "" {
			host = ingressLoadBalancer.Hostname
			break
		}
	}

	for _, rule := range kubeIngress.Spec.Rules {
		log.Printf("TRACE: ingress spec rule: %+v", rule)
		if rule.Host != "" {
			certHostname = rule.Host
			break
		}
	}

	return &Ingress{
		certificateARN: kubeIngress.getAnnotationsString(ingressCertificateARNAnnotation, ""),
		namespace:      kubeIngress.Metadata.Namespace,
		name:           kubeIngress.Metadata.Name,
		hostName:       host,
		ruleHostname:   certHostname,
	}
}

func newIngressForKube(i *Ingress) *ingress {
	return &ingress{
		Metadata: ingressItemMetadata{
			Namespace: i.namespace,
			Name:      i.name,
			Annotations: map[string]interface{}{
				ingressCertificateARNAnnotation: i.certificateARN,
			},
		},
		Status: ingressStatus{
			LoadBalancer: ingressLoadBalancerStatus{
				Ingress: []ingressLoadBalancer{
					{Hostname: i.hostName},
				},
			},
		},
	}
}

func newIngressMetadataForKube(i *Ingress) *ingress {
	return &ingress{
		Metadata: ingressItemMetadata{
			Namespace: i.namespace,
			Name:      i.name,
			Annotations: map[string]interface{}{
				ingressCertificateARNAnnotation: i.certificateARN,
			},
		},
	}
}

// NewAdapter creates an Adapter for Kubernetes using a given configuration.
func NewAdapter(config *Config) (*Adapter, error) {
	if config == nil || config.BaseURL == "" {
		return nil, ErrInvalidConfiguration
	}
	c, err := newSimpleClient(config)
	if err != nil {
		return nil, err
	}
	return &Adapter{kubeClient: c}, nil
}

// ListIngress can be used to obtain the list of ingress resources for all namespaces.
func (a *Adapter) ListIngress() ([]*Ingress, error) {
	il, err := listIngress(a.kubeClient) // Kubernetes ingress
	if err != nil {
		return nil, err
	}
	ret := make([]*Ingress, len(il.Items))
	for i, ingress := range il.Items {
		ret[i] = newIngressFromKube(ingress) // simplified Ingress
	}
	return ret, nil
}

// UpdateIngressLoadBalancer can be used to update the loadBalancer object of an ingress resource. It will update
// the hostname property with the provided load balancer DNS name.
func (a *Adapter) UpdateIngressLoadBalancer(ingress *Ingress, loadBalancerDNSName string) error {
	if ingress == nil || loadBalancerDNSName == "" {
		log.Printf("TRACE: UpdateIngressLoadBalancer invalid params")
		return ErrInvalidIngressUpdateParams
	}

	return updateIngressLoadBalancer(a.kubeClient, newIngressForKube(ingress), loadBalancerDNSName)
}

// UpdateIngressARN can be used to update the ARN of the Kubernetes
// ingress object to the ARN of the Ingress object.
func (a *Adapter) UpdateIngressARN(ingress *Ingress) error {
	if ingress == nil || ingress.CertificateARN() == "" {
		log.Printf("TRACE: UpdateIngressARN invalid params")
		return ErrInvalidIngressUpdateARNParams
	}

	return updateIngressARN(a.kubeClient, newIngressMetadataForKube(ingress), ingress.CertificateARN())
}
