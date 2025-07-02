package kubernetes

import (
	"errors"
	"fmt"
	"strings"

	elbv2Types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	log "github.com/sirupsen/logrus"
	"github.com/zalando-incubator/kube-ingress-aws-controller/aws"
	"k8s.io/client-go/kubernetes"
)

type Adapter struct {
	kubeClient                     client
	clientset                      kubernetes.Interface
	cniPodNamespace                string
	cniPodLabelSelector            string
	ingressClient                  *ingressClient
	ingressFilters                 []string
	ingressDefaultSecurityGroup    string
	ingressDefaultSSLPolicy        string
	ingressDefaultLoadBalancerType string
	clusterLocalDomain             string
	routeGroupSupport              bool
}

type IngressType string

const (
	TypeIngress    IngressType = "ingress"
	TypeRouteGroup IngressType = "routegroup"
)

const (
	DefaultClusterLocalDomain = ".cluster.local"
	loadBalancerTypeNLB       = "nlb"
	loadBalancerTypeALB       = "alb"
)

var (
	// ErrMissingKubernetesEnv is returned when the Kubernetes API server environment variables are not defined
	ErrMissingKubernetesEnv = errors.New("unable to load in-cluster configuration, KUBERNETES_SERVICE_HOST and KUBERNETES_SERVICE_PORT are not defined")
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
	ResourceType     IngressType
	Namespace        string
	Name             string
	Shared           bool
	HTTP2            bool
	ClusterLocal     bool
	CertificateARN   string
	Hostname         string
	Scheme           string
	SecurityGroup    string
	SSLPolicy        string
	IPAddressType    string
	LoadBalancerType string
	WAFWebACLID      string
	Hostnames        []string
}

// String returns a string representation of the Ingress instance containing the type, namespace and the resource name.
func (i *Ingress) String() string {
	return fmt.Sprintf("%s %s/%s", i.ResourceType, i.Namespace, i.Name)
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
func NewAdapter(config *Config, ingressAPIVersion string, ingressClassFilters []string, ingressDefaultSecurityGroup, ingressDefaultSSLPolicy, ingressDefaultLoadBalancerType, clusterLocalDomain string, disableInstrumentedHttpClient bool) (*Adapter, error) {
	if config == nil || config.BaseURL == "" {
		return nil, ErrInvalidConfiguration
	}
	c, err := newSimpleClient(config, disableInstrumentedHttpClient)
	if err != nil {
		return nil, err
	}

	return &Adapter{
		kubeClient:                     c,
		ingressClient:                  &ingressClient{apiVersion: ingressAPIVersion},
		ingressFilters:                 ingressClassFilters,
		ingressDefaultSecurityGroup:    ingressDefaultSecurityGroup,
		ingressDefaultSSLPolicy:        ingressDefaultSSLPolicy,
		ingressDefaultLoadBalancerType: loadBalancerTypesAWSToIngress[ingressDefaultLoadBalancerType],
		clusterLocalDomain:             clusterLocalDomain,
		routeGroupSupport:              true,
	}, nil
}

func (a *Adapter) newIngressFromKube(kubeIngress *ingress) (*Ingress, error) {
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

	return a.newIngress(TypeIngress, kubeIngress.Metadata, host, hostnames)
}

func (a *Adapter) newIngressFromRouteGroup(rg *routegroup) (*Ingress, error) {
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

	return a.newIngress(TypeRouteGroup, rg.Metadata, host, hostnames)
}

func (a *Adapter) newIngress(typ IngressType, metadata kubeItemMetadata, host string, hostnames []string) (*Ingress, error) {
	annotations := metadata.Annotations

	var scheme elbv2Types.LoadBalancerSchemeEnum
	// Set schema to default if annotation value is not valid
	annotationValue := getAnnotationsString(annotations, ingressSchemeAnnotation, "")
	switch elbv2Types.LoadBalancerSchemeEnum(annotationValue) {
	case elbv2Types.LoadBalancerSchemeEnumInternal:
		scheme = elbv2Types.LoadBalancerSchemeEnumInternal
	default:
		scheme = elbv2Types.LoadBalancerSchemeEnumInternetFacing
	}

	shared := true
	if getAnnotationsString(annotations, ingressSharedAnnotation, "") == "false" {
		shared = false
	}

	ipAddressType := aws.IPAddressTypeDualstack
	if getAnnotationsString(annotations, ingressALBIPAddressType, "") == aws.IPAddressTypeIPV4 {
		ipAddressType = aws.IPAddressTypeIPV4
	}

	sslPolicy := getAnnotationsString(annotations, ingressSSLPolicyAnnotation, a.ingressDefaultSSLPolicy)
	if _, ok := aws.SSLPolicies[sslPolicy]; !ok {
		sslPolicy = a.ingressDefaultSSLPolicy
	}

	loadBalancerType, hasLB := annotations[ingressLoadBalancerTypeAnnotation]
	if !hasLB {
		// internal load balancers should be ALB if user do not override the decision
		// https://docs.aws.amazon.com/elasticloadbalancing/latest/network/load-balancer-troubleshooting.html#intermittent-connection-failure
		if scheme == elbv2Types.LoadBalancerSchemeEnumInternal {
			loadBalancerType = loadBalancerTypeALB
		} else {
			loadBalancerType = a.ingressDefaultLoadBalancerType
		}
	}

	securityGroup, hasSG := annotations[ingressSecurityGroupAnnotation]
	if !hasSG {
		securityGroup = a.ingressDefaultSecurityGroup
	}

	wafWebAclId, hasWAF := annotations[ingressWAFWebACLIDAnnotation]

	if (loadBalancerType == loadBalancerTypeNLB) && (hasSG || hasWAF) {
		if hasLB {
			return nil, errors.New("security group or WAF are not supported by NLB (configured by annotation)")
		}
		// Security Group or WAF are not supported by NLB (default), falling back to ALB
		loadBalancerType = loadBalancerTypeALB
	}

	if _, ok := loadBalancerTypesIngressToAWS[loadBalancerType]; !ok {
		loadBalancerType = a.ingressDefaultLoadBalancerType
	}

	// convert to the internal naming e.g. nlb -> network
	loadBalancerType = loadBalancerTypesIngressToAWS[loadBalancerType]

	if loadBalancerType == aws.LoadBalancerTypeNetwork {
		// ensure both ipv4 and ipv6 for network load balancers
		ipAddressType = aws.IPAddressTypeDualstack
	}

	http2 := true
	if getAnnotationsString(annotations, ingressHTTP2Annotation, "") == "false" {
		http2 = false
	}

	return &Ingress{
		ResourceType:     typ,
		Namespace:        metadata.Namespace,
		Name:             metadata.Name,
		Hostname:         host,
		Hostnames:        hostnames,
		ClusterLocal:     len(hostnames) < 1,
		CertificateARN:   getAnnotationsString(annotations, ingressCertificateARNAnnotation, ""),
		Scheme:           string(scheme),
		Shared:           shared,
		SecurityGroup:    securityGroup,
		SSLPolicy:        sslPolicy,
		IPAddressType:    ipAddressType,
		LoadBalancerType: loadBalancerType,
		WAFWebACLID:      wafWebAclId,
		HTTP2:            http2,
	}, nil
}

// Get ingress class filters that are used to filter ingresses acted upon.
func (a *Adapter) IngressFiltersString() string {
	return strings.TrimSpace(strings.Join(a.ingressFilters, ","))
}

// ListResources can be used to obtain the list of ingress and routegroup
// resources for all namespaces filtered by class. It
// returns the Ingress business object, that for the controller does
// not matter to be routegroup or ingress..
func (a *Adapter) ListResources() ([]*Ingress, error) {
	ings, err := a.ListIngress()
	if err != nil {
		return nil, err
	}

	var rgs []*Ingress
	if a.routeGroupSupport {
		rgs, err = a.ListRoutegroups()
		if err != nil {
			if errors.Is(err, ErrResourceNotFound) || errors.Is(err, ErrNoPermissionToAccessResource) {
				a.routeGroupSupport = false
				log.Warnf("Disabling RouteGroup support because listing RouteGroups failed: %v, to get more information https://opensource.zalando.com/skipper/kubernetes/routegroups/#routegroups", err)
			} else {
				// Generic error, RouteGroup CRD exists and we have permission to access
				return nil, err
			}
		}
	}

	ings = append(ings, rgs...)
	return ings, nil
}

// ListIngress can be used to obtain the list of ingress resources for
// all namespaces filtered by class. It returns the Ingress business
// object, that for the controller does not matter to be
// routegroup or ingress..
func (a *Adapter) ListIngress() ([]*Ingress, error) {
	il, err := a.ingressClient.listIngress(a.kubeClient)
	if err != nil {
		return nil, err
	}
	var ret []*Ingress
	for _, ingress := range il.Items {
		if !a.supportedIngress(ingress) {
			continue
		}
		ing, err := a.newIngressFromKube(ingress)
		if err == nil {
			ret = append(ret, ing)
		} else {
			log.WithFields(log.Fields{
				"type": TypeIngress,
				"ns":   ingress.Metadata.Namespace,
				"name": ingress.Metadata.Name,
			}).Errorf("%v", err)
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
	for _, rg := range rgs.Items {
		if !a.supportedCRD(rg.Metadata) {
			continue
		}
		ing, err := a.newIngressFromRouteGroup(rg)
		if err == nil {
			ret = append(ret, ing)
		} else {
			log.WithFields(log.Fields{
				"type": TypeRouteGroup,
				"ns":   rg.Metadata.Namespace,
				"name": rg.Metadata.Name,
			}).Errorf("%v", err)
		}
	}
	return ret, nil
}

func (a *Adapter) supportedCRD(metadata kubeItemMetadata) bool {
	if len(a.ingressFilters) == 0 {
		return true
	}
	ingressClass := getAnnotationsString(metadata.Annotations, ingressClassAnnotation, "")
	return a.supportedIngressClass(ingressClass)
}

func (a *Adapter) supportedIngress(ingress *ingress) bool {
	if len(a.ingressFilters) == 0 {
		return true
	}
	ingressClass := getIngressClassName(ingress.Spec, "")
	// fallback to deprecated annotation
	// https://kubernetes.io/docs/concepts/services-networking/ingress/#deprecated-annotation
	if ingressClass == "" {
		ingressClass = getAnnotationsString(ingress.Metadata.Annotations, ingressClassAnnotation, "")
	}
	return a.supportedIngressClass(ingressClass)
}

func (a *Adapter) supportedIngressClass(ingressClass string) bool {
	for _, v := range a.ingressFilters {
		if v == ingressClass {
			return true
		}
	}
	return false
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

	if ingress.Hostname == loadBalancerDNSName {
		return ErrUpdateNotNeeded
	}

	switch ingress.ResourceType {
	case TypeRouteGroup:
		return updateRoutegroupLoadBalancer(a.kubeClient, ingress.Namespace, ingress.Name, loadBalancerDNSName)
	case TypeIngress:
		return a.ingressClient.updateIngressLoadBalancer(a.kubeClient, ingress.Namespace, ingress.Name, loadBalancerDNSName)
	}
	return fmt.Errorf("unknown resourceType '%s', failed to update Kubernetes resource", ingress.ResourceType)
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

// WithTargetCNIPodSelector returns the receiver adapter after setting
// the TargetCNIPodSelector config.
func (a *Adapter) WithTargetCNIPodSelector(ns string, selector string) *Adapter {
	a.cniPodNamespace, a.cniPodLabelSelector = ns, selector
	return a
}
