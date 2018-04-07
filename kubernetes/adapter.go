package kubernetes

import (
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go/service/elbv2"
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
	// ErrInvalidCertificates is returned when the CA certificates required to communicate with the
	// API server are invalid
	ErrInvalidCertificates = errors.New("invalid CA certificates")
)

// Ingress is the ingress-controller's business object
type Ingress struct {
	certificateARN string
	namespace      string
	name           string
	hostName       string
	scheme         string
	hostnames      []string
	shared         bool
}

// CertificateARN returns the AWS certificate (IAM or ACM) ARN found in the ingress resource metadata.
// It returns an empty string if the annotation is missing.
func (i *Ingress) CertificateARN() string {
	return i.certificateARN
}

// String returns a string representation of the Ingress instance containing the namespace and the resource name.
func (i *Ingress) String() string {
	return fmt.Sprintf("%s/%s", i.namespace, i.name)
}

// Hostname returns the DNS LoadBalancer hostname associated with the
// ingress gotten from Kubernetes Status
func (i *Ingress) Hostname() string {
	return i.hostName
}

// Scheme returns the scheme associated with the ingress
func (i *Ingress) Scheme() string {
	return i.scheme
}

// Hostnames returns the DNS hostnames associated with the ingress
// gotten from Kubernetes Spec
func (i *Ingress) Hostnames() []string {
	return i.hostnames
}

// SetScheme sets Ingress.scheme to the scheme as specified.
func (i *Ingress) SetScheme(scheme string) {
	i.scheme = scheme
}

// Shared return true if the ingress can share ALB with other ingresses.
func (i *Ingress) Shared() bool {
	return i.shared
}

// Name returns the ingress name.
func (i *Ingress) Name() string {
	return i.name
}

// Namespace returns the ingress namespace.
func (i *Ingress) Namespace() string {
	return i.namespace
}

func newIngressFromKube(kubeIngress *ingress) *Ingress {
	var host, scheme string
	var hostnames []string
	for _, ingressLoadBalancer := range kubeIngress.Status.LoadBalancer.Ingress {
		if ingressLoadBalancer.Hostname != "" {
			host = ingressLoadBalancer.Hostname
			break
		}
	}

	for _, rule := range kubeIngress.Spec.Rules {
		if rule.Host != "" {
			hostnames = append(hostnames, rule.Host)
		}
	}

	// Set schema to default if annotation value is not valid
	switch kubeIngress.getAnnotationsString(ingressSchemeAnnotation, "") {
	case elbv2.LoadBalancerSchemeEnumInternal:
		scheme = elbv2.LoadBalancerSchemeEnumInternal
	default:
		scheme = elbv2.LoadBalancerSchemeEnumInternetFacing
	}

	certDomain := kubeIngress.getAnnotationsString(ingressCertificateDomainAnnotation, "")
	if certDomain != "" {
		hostnames = []string{certDomain}
	}

	shared := true

	sharedValue := kubeIngress.getAnnotationsString(ingressSharedAnnotation, "")
	if sharedValue == "false" {
		shared = false
	}

	return &Ingress{
		certificateARN: kubeIngress.getAnnotationsString(ingressCertificateARNAnnotation, ""),
		namespace:      kubeIngress.Metadata.Namespace,
		name:           kubeIngress.Metadata.Name,
		hostName:       host,
		scheme:         scheme,
		hostnames:      hostnames,
		shared:         shared,
	}
}

func newIngressForKube(i *Ingress) *ingress {
	shared := "true"

	if !i.shared {
		shared = "false"
	}

	return &ingress{
		Metadata: ingressItemMetadata{
			Namespace: i.namespace,
			Name:      i.name,
			Annotations: map[string]interface{}{
				ingressCertificateARNAnnotation: i.certificateARN,
				ingressSchemeAnnotation:         i.scheme,
				ingressSharedAnnotation:         shared,
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
	il, err := listIngress(a.kubeClient)
	if err != nil {
		return nil, err
	}
	ret := make([]*Ingress, len(il.Items))
	for i, ingress := range il.Items {
		ret[i] = newIngressFromKube(ingress)
	}
	return ret, nil
}

// UpdateIngressLoadBalancer can be used to update the loadBalancer object of an ingress resource. It will update
// the hostname property with the provided load balancer DNS name.
func (a *Adapter) UpdateIngressLoadBalancer(ingress *Ingress, loadBalancerDNSName string) error {
	if ingress == nil || loadBalancerDNSName == "" {
		return ErrInvalidIngressUpdateParams
	}

	return updateIngressLoadBalancer(a.kubeClient, newIngressForKube(ingress), loadBalancerDNSName)
}
