package kubernetes

import (
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/service/elbv2"
	log "github.com/sirupsen/logrus"
	"github.com/zalando-incubator/kube-ingress-aws-controller/aws"
)

type Adapter struct {
	kubeClient                     client
	ingressFilters                 []string
	ingressDefaultSecurityGroup    string
	ingressDefaultSSLPolicy        string
	ingressDefaultLoadBalancerType string
	clusterLocalDomain             string
	routeGroupSupport              bool
}

type ingressType int

const (
	ingressTypeIngress ingressType = iota + 1
	ingressTypeRouteGroup
)

func (t ingressType) String() string {
	switch t {
	case ingressTypeIngress:
		return "ingress"
	case ingressTypeRouteGroup:
		return "routegroup"
	default:
		return "unknown"
	}
}

const (
	DefaultClusterLocalDomain = ".cluster.local"
	loadBalancerTypeNLB       = "nlb"
	loadBalancerTypeALB       = "alb"
)

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

	loadBalancerTypesIngressToAWS = map[string]string{
		loadBalancerTypeALB: aws.LoadBalancerTypeApplication,
		loadBalancerTypeNLB: aws.LoadBalancerTypeNetwork,
	}

	loadBalancerTypesAWSToIngress = map[string]string{
		aws.LoadBalancerTypeApplication: loadBalancerTypeALB,
		aws.LoadBalancerTypeNetwork:     loadBalancerTypeNLB,
	}
)

// Ingress is the ingress-controller's business object. It is used to
// store Kubernetes ingress and routegroup resources.
type Ingress struct {
	Shared           bool
	HTTP2            bool
	ClusterLocal     bool
	CertificateARN   string
	Namespace        string
	Name             string
	Hostname         string
	Scheme           string
	SecurityGroup    string
	SSLPolicy        string
	IPAddressType    string
	LoadBalancerType string
	WAFWebACLId      string
	Hostnames        []string
	resourceType     ingressType
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
func NewAdapter(config *Config, ingressClassFilters []string, ingressDefaultSecurityGroup, ingressDefaultSSLPolicy, ingressDefaultLoadBalancerType, clusterLocalDomain string) (*Adapter, error) {
	if config == nil || config.BaseURL == "" {
		return nil, ErrInvalidConfiguration
	}
	c, err := newSimpleClient(config)
	if err != nil {
		return nil, err
	}
	return &Adapter{
		kubeClient:                     c,
		ingressFilters:                 ingressClassFilters,
		ingressDefaultSecurityGroup:    ingressDefaultSecurityGroup,
		ingressDefaultSSLPolicy:        ingressDefaultSSLPolicy,
		ingressDefaultLoadBalancerType: loadBalancerTypesAWSToIngress[ingressDefaultLoadBalancerType],
		clusterLocalDomain:             clusterLocalDomain,
		routeGroupSupport:              true,
	}, nil
}

func (a *Adapter) newIngressFromKube(kubeIngress *ingress) *Ingress {
	var host string
	var hostnames []string
	for _, ingressLoadBalancer := range kubeIngress.Status.LoadBalancer.Ingress {
		if ingressLoadBalancer.Hostname != "" {
			host = ingressLoadBalancer.Hostname
			break
		}
	}

	for _, rule := range kubeIngress.Spec.Rules {
		if rule.Host != "" && (a.clusterLocalDomain == "" || !strings.HasSuffix(rule.Host, a.clusterLocalDomain)) {
			hostnames = append(hostnames, rule.Host)
		}
	}

	ingress := a.parseAnnotations(kubeIngress.Metadata.Annotations)

	ingress.Namespace = kubeIngress.Metadata.Namespace
	ingress.Name = kubeIngress.Metadata.Name
	ingress.Hostname = host
	ingress.Hostnames = hostnames
	ingress.resourceType = ingressTypeIngress
	ingress.ClusterLocal = len(hostnames) < 1

	return ingress
}

func (a *Adapter) newIngressFromRouteGroup(rg *routegroup) *Ingress {
	var host string
	var hostnames []string
	for _, lb := range rg.Status.LoadBalancer.Routegroup {
		if lb.Hostname != "" {
			host = lb.Hostname
			break
		}
	}

	for _, host := range rg.Spec.Hosts {
		if host != "" && (a.clusterLocalDomain == "" || !strings.HasSuffix(host, a.clusterLocalDomain)) {
			hostnames = append(hostnames, host)
		}
	}

	ingress := a.parseAnnotations(rg.Metadata.Annotations)

	ingress.Namespace = rg.Metadata.Namespace
	ingress.Name = rg.Metadata.Name
	ingress.Hostname = host
	ingress.Hostnames = hostnames
	ingress.resourceType = ingressTypeRouteGroup
	ingress.ClusterLocal = len(hostnames) < 1

	return ingress
}

// parseAnnotations parses the ingress configuration from the annotations of an
// Ingress or ReouteGroup resource.
func (a *Adapter) parseAnnotations(annotations map[string]string) *Ingress {
	var scheme string
	// Set schema to default if annotation value is not valid
	switch getAnnotationsString(annotations, ingressSchemeAnnotation, "") {
	case elbv2.LoadBalancerSchemeEnumInternal:
		scheme = elbv2.LoadBalancerSchemeEnumInternal
	default:
		scheme = elbv2.LoadBalancerSchemeEnumInternetFacing
	}

	shared := true
	if getAnnotationsString(annotations, ingressSharedAnnotation, "") == "false" {
		shared = false
	}

	ipAddressType := aws.IPAddressTypeIPV4
	if getAnnotationsString(annotations, ingressALBIPAddressType, "") == aws.IPAddressTypeDualstack {
		ipAddressType = aws.IPAddressTypeDualstack
	}

	sslPolicy := getAnnotationsString(annotations, ingressSSLPolicyAnnotation, a.ingressDefaultSSLPolicy)
	if _, ok := aws.SSLPolicies[sslPolicy]; !ok {
		sslPolicy = a.ingressDefaultSSLPolicy
	}

	loadBalancerType := getAnnotationsString(annotations, ingressLoadBalancerTypeAnnotation, a.ingressDefaultLoadBalancerType)
	if _, ok := loadBalancerTypesIngressToAWS[loadBalancerType]; !ok {
		loadBalancerType = a.ingressDefaultLoadBalancerType
	}

	// convert to the internal naming e.g. nlb -> network
	loadBalancerType = loadBalancerTypesIngressToAWS[loadBalancerType]

	if loadBalancerType == aws.LoadBalancerTypeNetwork {
		// ensure ipv4 for network load balancers
		ipAddressType = aws.IPAddressTypeIPV4
	}

	http2 := true
	if getAnnotationsString(annotations, ingressHTTP2Annotation, "") == "false" {
		http2 = false
	}

	return &Ingress{
		CertificateARN:   getAnnotationsString(annotations, ingressCertificateARNAnnotation, ""),
		Scheme:           scheme,
		Shared:           shared,
		SecurityGroup:    getAnnotationsString(annotations, ingressSecurityGroupAnnotation, a.ingressDefaultSecurityGroup),
		SSLPolicy:        sslPolicy,
		IPAddressType:    ipAddressType,
		LoadBalancerType: loadBalancerType,
		WAFWebACLId:      getAnnotationsString(annotations, ingressWAFWebACLIdAnnotation, ""),
		HTTP2:            http2,
	}
}

func newMetadataForKube(i *Ingress) kubeItemMetadata {
	shared := "true"
	if !i.Shared {
		shared = "false"
	}

	http2 := "true"
	if !i.HTTP2 {
		http2 = "false"
	}

	return kubeItemMetadata{
		Namespace: i.Namespace,
		Name:      i.Name,
		Annotations: map[string]string{
			ingressCertificateARNAnnotation:   i.CertificateARN,
			ingressSchemeAnnotation:           i.Scheme,
			ingressSharedAnnotation:           shared,
			ingressHTTP2Annotation:            http2,
			ingressSecurityGroupAnnotation:    i.SecurityGroup,
			ingressSSLPolicyAnnotation:        i.SSLPolicy,
			ingressALBIPAddressType:           i.IPAddressType,
			ingressWAFWebACLIdAnnotation:      i.WAFWebACLId,
			ingressLoadBalancerTypeAnnotation: loadBalancerTypesAWSToIngress[i.LoadBalancerType],
		},
	}
}

func newIngressForKube(i *Ingress) *ingress {
	return &ingress{
		Metadata: newMetadataForKube(i),
		Status: ingressStatus{
			LoadBalancer: ingressLoadBalancerStatus{
				Ingress: []ingressLoadBalancer{
					{Hostname: i.Hostname},
				},
			},
		},
	}
}

func newRouteGroupForKube(i *Ingress) *routegroup {
	return &routegroup{
		Metadata: newMetadataForKube(i),
		Status: routegroupStatus{
			LoadBalancer: routegroupLoadBalancerStatus{
				Routegroup: []routegroupLoadBalancer{
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

// ListResources can be used to obtain the list of ingress and
// routegroup resources for all namespaces filtered by class. It
// returns the Ingress business object, that for the controller does
// not matter to be routegroup or ingress..
func (a *Adapter) ListResources() ([]*Ingress, error) {
	ings, err := a.ListIngress()
	if err != nil {
		return nil, err
	}
	rgs, err := a.ListRoutegroups()
	if err != nil {
		if a.routeGroupSupport {
			a.routeGroupSupport = false
			log.Warnf("Failed to list RouteGroups: %v, to get more information https://opensource.zalando.com/skipper/kubernetes/routegroups/#routegroups", err)
		}
		// RouteGroup CRD does not exist
		if err == ErrRessourceNotFound {
			return ings, nil
		}
		return nil, err
	}
	a.routeGroupSupport = true
	return append(ings, rgs...), nil
}

// ListIngress can be used to obtain the list of ingress resources for
// all namespaces filtered by class. It returns the Ingress business
// object, that for the controller does not matter to be routegroup or
// ingress..
func (a *Adapter) ListIngress() ([]*Ingress, error) {
	il, err := listIngress(a.kubeClient)
	if err != nil {
		return nil, err
	}
	var ret []*Ingress
	if len(a.ingressFilters) > 0 {
		for _, ingress := range il.Items {
			ingressClass := getAnnotationsString(ingress.Metadata.Annotations, ingressClassAnnotation, "")
			for _, v := range a.ingressFilters {
				if v == ingressClass {
					ret = append(ret, a.newIngressFromKube(ingress))
				}
			}
		}
	} else {
		for _, ingress := range il.Items {
			ret = append(ret, a.newIngressFromKube(ingress))
		}
	}
	return ret, nil
}

// ListRoutegroups can be used to obtain the list of Ingress resources
// for all namespaces filtered by class. It returns the Ingress
// business object, that for the controller does not matter to be
// routegroup or ingress.
func (a *Adapter) ListRoutegroups() ([]*Ingress, error) {
	rgs, err := listRoutegroups(a.kubeClient)
	if err != nil {
		return nil, err
	}

	var ret []*Ingress
	if len(a.ingressFilters) > 0 {
		for _, rg := range rgs.Items {
			ingressClass := getAnnotationsString(rg.Metadata.Annotations, ingressClassAnnotation, "")
			for _, v := range a.ingressFilters {
				if v == ingressClass {
					ret = append(ret, a.newIngressFromRouteGroup(rg))
				}
			}
		}
	} else {
		for _, rg := range rgs.Items {
			ret = append(ret, a.newIngressFromRouteGroup(rg))
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

	if loadBalancerDNSName == DefaultClusterLocalDomain {
		loadBalancerDNSName = ""
	}

	switch ingress.resourceType {
	case ingressTypeRouteGroup:
		return updateRoutegroupLoadBalancer(a.kubeClient, newRouteGroupForKube(ingress), loadBalancerDNSName)
	case ingressTypeIngress:
		return updateIngressLoadBalancer(a.kubeClient, newIngressForKube(ingress), loadBalancerDNSName)
	}
	return fmt.Errorf("Unknown resourceType '%s', failed to update Kubernetes resource", ingress.resourceType)
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
