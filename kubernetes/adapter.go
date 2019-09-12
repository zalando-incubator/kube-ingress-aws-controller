package kubernetes

import (
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/service/elbv2"
)

type Adapter struct {
	kubeClient                  client
	ingressFilters              []string
	ingressDefaultSecurityGroup string
	ingressDefaultSSLPolicy     string
}

const ipAddressTypeDualstack = "dualstack"

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
	CertificateARN string
	Namespace      string
	Name           string
	Hostname       string
	Scheme         string
	Hostnames      []string
	Shared         bool
	SecurityGroup  string
	SSLPolicy      string
	IPAddressType  string
}

// String returns a string representation of the Ingress instance containing the namespace and the resource name.
func (i *Ingress) String() string {
	return fmt.Sprintf("%s/%s", i.Namespace, i.Name)
}

// ConfigMap is the ingress-controller's representation of a Kubernetes
// ConfigMap
type ConfigMap struct {
	Namespace string
	Name      string
	Data      map[string]string
}

// String implements fmt.Stringer.
func (c *ConfigMap) String() string {
	return fmt.Sprintf("%s/%s", c.Namespace, c.Name)
}

// NewAdapter creates an Adapter for Kubernetes using a given configuration.
func NewAdapter(config *Config, ingressClassFilters []string, ingressDefaultSecurityGroup, ingressDefaultSSLPolicy string) (*Adapter, error) {
	if config == nil || config.BaseURL == "" {
		return nil, ErrInvalidConfiguration
	}
	c, err := newSimpleClient(config)
	if err != nil {
		return nil, err
	}
	return &Adapter{
		kubeClient:                  c,
		ingressFilters:              ingressClassFilters,
		ingressDefaultSecurityGroup: ingressDefaultSecurityGroup,
		ingressDefaultSSLPolicy:     ingressDefaultSSLPolicy,
	}, nil
}

func (a *Adapter) newIngressFromKube(kubeIngress *ingress) *Ingress {
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

	shared := true
	if kubeIngress.getAnnotationsString(ingressSharedAnnotation, "") == "false" {
		shared = false
	}

	ipAddressType := "ipv4"
	if kubeIngress.getAnnotationsString(ingressALBIPAddressType, "") == ipAddressTypeDualstack {
		ipAddressType = ipAddressTypeDualstack
	}

	return &Ingress{
		CertificateARN: kubeIngress.getAnnotationsString(ingressCertificateARNAnnotation, ""),
		Namespace:      kubeIngress.Metadata.Namespace,
		Name:           kubeIngress.Metadata.Name,
		Hostname:       host,
		Scheme:         scheme,
		Hostnames:      hostnames,
		Shared:         shared,
		SecurityGroup:  kubeIngress.getAnnotationsString(ingressSecurityGroupAnnotation, a.ingressDefaultSecurityGroup),
		SSLPolicy:      kubeIngress.getAnnotationsString(ingressSSLPolicyAnnotation, a.ingressDefaultSSLPolicy),
		IPAddressType:  ipAddressType,
	}
}

func newIngressForKube(i *Ingress) *ingress {
	shared := "true"

	if !i.Shared {
		shared = "false"
	}

	return &ingress{
		Metadata: ingressItemMetadata{
			Namespace: i.Namespace,
			Name:      i.Name,
			Annotations: map[string]interface{}{
				ingressCertificateARNAnnotation: i.CertificateARN,
				ingressSchemeAnnotation:         i.Scheme,
				ingressSharedAnnotation:         shared,
				ingressSecurityGroupAnnotation:  i.SecurityGroup,
				ingressSSLPolicyAnnotation:      i.SSLPolicy,
				ingressALBIPAddressType:         i.IPAddressType,
			},
		},
		Status: ingressStatus{
			LoadBalancer: ingressLoadBalancerStatus{
				Ingress: []ingressLoadBalancer{
					{Hostname: i.Hostname},
				},
			},
		},
	}
}

// Get ingress class filters that are used to filter ingresses acted upon.
func (a *Adapter) IngressFiltersString() string {
	return strings.TrimSpace(strings.Join(a.ingressFilters, ","))
}

// ListIngress can be used to obtain the list of ingress resources for all namespaces.
func (a *Adapter) ListIngress() ([]*Ingress, error) {
	il, err := listIngress(a.kubeClient)
	if err != nil {
		return nil, err
	}
	var ret []*Ingress
	if len(a.ingressFilters) > 0 {
		ret = make([]*Ingress, 0)
		for _, ingress := range il.Items {
			ingressClass := ingress.getAnnotationsString(ingressClassAnnotation, "")
			if ingressClass != "" {
				for _, v := range a.ingressFilters {
					if v == ingressClass {
						ret = append(ret, a.newIngressFromKube(ingress))
					}
				}
			}
		}
	} else {
		ret = make([]*Ingress, len(il.Items))
		for i, ingress := range il.Items {
			ret[i] = a.newIngressFromKube(ingress)
		}
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

// GetConfigMap retrieves the ConfigMap with name from namespace.
func (a *Adapter) GetConfigMap(namespace, name string) (*ConfigMap, error) {
	cm, err := getConfigMap(a.kubeClient, namespace, name)
	if err != nil {
		return nil, err
	}

	return &ConfigMap{
		Name:      cm.Metadata.Name,
		Namespace: cm.Metadata.Namespace,
		Data:      cm.Data,
	}, nil
}
