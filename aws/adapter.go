package aws

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2Types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/linki/instrumented_http"
	log "github.com/sirupsen/logrus"
	"github.com/zalando-incubator/kube-ingress-aws-controller/certs"
	"github.com/zalando-incubator/kube-ingress-aws-controller/problem"
)

// An Adapter can be used to orchestrate and obtain information from Amazon Web Services.
type Adapter struct {
	ec2metadata    *imds.Client
	ec2            EC2API
	elbv2          ELBV2API
	autoscaling    AutoScalingAPI
	acm            ACMAPI
	iam            IAMAPI
	cloudformation CloudFormationAPI

	manifest                    *manifest
	healthCheckPath             string
	healthCheckPort             uint
	healthCheckInterval         time.Duration
	healthCheckTimeout          time.Duration
	albHealthyThresholdCount    uint
	albUnhealthyThresholdCount  uint
	nlbHealthyThresholdCount    uint
	targetType                  elbv2Types.TargetTypeEnum
	targetPort                  uint
	albHTTPTargetPort           uint
	nlbHTTPTargetPort           uint
	targetHTTPS                 bool
	creationTimeout             time.Duration
	idleConnectionTimeout       time.Duration
	deregistrationDelayTimeout  time.Duration
	TargetedAutoScalingGroups   map[string]*autoScalingGroupDetails
	OwnedAutoScalingGroups      map[string]*autoScalingGroupDetails
	ec2Details                  map[string]*instanceDetails
	singleInstances             map[string]*instanceDetails
	obsoleteInstances           []string
	stackTerminationProtection  bool
	stackTags                   map[string]string
	controllerID                string
	sslPolicy                   string
	ipAddressType               string
	albLogsS3Bucket             string
	albLogsS3Prefix             string
	nlbZoneAffinity             string
	httpRedirectToHTTPS         bool
	nlbCrossZone                bool
	nlbHTTPEnabled              bool
	customFilter                string
	internalDomains             []string
	denyInternalDomains         bool
	denyInternalRespBody        string
	denyInternalRespContentType string
	denyInternalRespStatusCode  int
	TargetCNI                   *TargetCNIconfig
}

type TargetCNIconfig struct {
	Enabled       bool
	TargetGroupCh chan []string
}

type manifest struct {
	securityGroup *securityGroupDetails
	instance      *instanceDetails
	subnets       []*subnetDetails
	filters       []ec2Types.Filter
	asgFilters    map[string][]string
	clusterID     string
	vpcID         string
}

const (
	DefaultHealthCheckPath            = "/kube-system/healthz"
	DefaultHealthCheckPort            = 9999
	DefaultTargetPort                 = 9999
	DefaultHealthCheckInterval        = 10 * time.Second
	DefaultHealthCheckTimeout         = 5 * time.Second
	DefaultAlbHealthyThresholdCount   = 5
	DefaultAlbUnhealthyThresholdCount = 2
	DefaultNlbHealthyThresholdCount   = 3
	DefaultCertificateUpdateInterval  = 30 * time.Minute
	DefaultCreationTimeout            = 5 * time.Minute
	DefaultIdleConnectionTimeout      = 1 * time.Minute
	DefaultDeregistrationTimeout      = 5 * time.Minute
	DefaultControllerID               = "kube-ingress-aws-controller"
	// DefaultMaxCertsPerALB defines the maximum number of certificates per
	// ALB. AWS limit is 25 but one space is needed to work around
	// CloudFormation bug:
	// https://github.com/zalando-incubator/kube-ingress-aws-controller/pull/162
	DefaultMaxCertsPerALB = 24
	// DefaultSslPolicy defines the set of protocols and ciphers that will be
	// accepted by an SSL endpoint.
	// See; https://docs.aws.amazon.com/elasticloadbalancing/latest/application/create-https-listener.html#describe-ssl-policies
	DefaultSslPolicy = "ELBSecurityPolicy-2016-08"
	// DefaultIpAddressType sets IpAddressType to "ipv4", it is either ipv4 or dualstack
	DefaultIpAddressType = "ipv4"
	// DefaultAlbS3LogsBucket is a blank string, and must be set if enabled
	DefaultAlbS3LogsBucket = ""
	// DefaultAlbS3LogsPrefix is a blank string, and optionally set if desired
	DefaultAlbS3LogsPrefix = ""

	DefaultCustomFilter = ""

	// DefaultZoneAffinity specifies dns_record.client_routing_policy, see also https://docs.aws.amazon.com/elasticloadbalancing/latest/network/network-load-balancers.html#load-balancer-attributes
	DefaultZoneAffinity = "any_availability_zone"

	// DefaultNLBCrossZone specifies the default configuration for cross
	// zone load balancing: https://docs.aws.amazon.com/elasticloadbalancing/latest/network/network-load-balancers.html#load-balancer-attributes
	// It it is safe to change as per https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-loadbalancer-loadbalancerattribute.html#aws-properties-elasticloadbalancingv2-loadbalancer-loadbalancerattribute-properties
	DefaultNLBCrossZone   = false
	DefaultNLBHTTPEnabled = false

	nameTag                     = "Name"
	LoadBalancerTypeApplication = "application"
	LoadBalancerTypeNetwork     = "network"
	IPAddressTypeIPV4           = "ipv4"
	IPAddressTypeDualstack      = "dualstack"

	TargetAccessModeAWSCNI   = "AWSCNI"
	TargetAccessModeHostPort = "HostPort"
	TargetAccessModeLegacy   = "Legacy"
)

var (
	// ErrLoadBalancerStackNotFound is used to signal that a given load balancer CF stack was not found.
	ErrLoadBalancerStackNotFound = errors.New("load balancer stack not found")
	// ErrLoadBalancerStackNotReady is used to signal that a given load balancer CF stack is not ready to be used.
	ErrLoadBalancerStackNotReady = errors.New("existing load balancer stack not ready")
	// ErrMissingNameTag is used to signal that the Name tag on a given resource is missing.
	ErrMissingNameTag = errors.New("Name tag not found")
	// ErrMissingTag is used to signal that a tag on a given resource is missing.
	ErrMissingTag = errors.New("missing tag")
	// ErrNoSubnets is used to signal that no subnets were found in the current VPC
	ErrNoSubnets = errors.New("unable to find VPC subnets")
	// ErrMissingAutoScalingGroupTag is used to signal that the auto scaling group tag is not present in the list of tags.
	ErrMissingAutoScalingGroupTag = errors.New(`instance is missing the "` + autoScalingGroupNameTag + `" tag`)
	// ErrNoRunningInstances is used to signal that no instances were found in the running state
	ErrNoRunningInstances = errors.New("no reservations or instances in the running state")
	// SSLPolicies is a map of valid ALB SSL Policies
	// https://docs.aws.amazon.com/elasticloadbalancing/latest/application/create-https-listener.html#describe-ssl-policies
	SSLPolicies = map[string]bool{
		"ELBSecurityPolicy-2016-08":                true,
		"ELBSecurityPolicy-FS-2018-06":             true,
		"ELBSecurityPolicy-TLS-1-2-2017-01":        true,
		"ELBSecurityPolicy-TLS-1-2-Ext-2018-06":    true,
		"ELBSecurityPolicy-TLS-1-1-2017-01":        true,
		"ELBSecurityPolicy-2015-05":                true,
		"ELBSecurityPolicy-TLS-1-0-2015-04":        true,
		"ELBSecurityPolicy-FS-1-1-2019-08":         true,
		"ELBSecurityPolicy-FS-1-2-2019-08":         true,
		"ELBSecurityPolicy-FS-1-2-Res-2019-08":     true,
		"ELBSecurityPolicy-FS-1-2-Res-2020-10":     true,
		"ELBSecurityPolicy-TLS13-1-2-2021-06":      true,
		"ELBSecurityPolicy-TLS13-1-2-Res-2021-06":  true,
		"ELBSecurityPolicy-TLS13-1-2-Ext1-2021-06": true,
		"ELBSecurityPolicy-TLS13-1-2-Ext2-2021-06": true,
		"ELBSecurityPolicy-TLS13-1-1-2021-06":      true,
		"ELBSecurityPolicy-TLS13-1-0-2021-06":      true,
		"ELBSecurityPolicy-TLS13-1-3-2021-06":      true,
	}
	SSLPoliciesList = []string{
		"ELBSecurityPolicy-2016-08",
		"ELBSecurityPolicy-FS-2018-06",
		"ELBSecurityPolicy-TLS-1-2-2017-01",
		"ELBSecurityPolicy-TLS-1-2-Ext-2018-06",
		"ELBSecurityPolicy-TLS-1-1-2017-01",
		"ELBSecurityPolicy-2015-05",
		"ELBSecurityPolicy-TLS-1-0-2015-04",
		"ELBSecurityPolicy-FS-1-1-2019-08",
		"ELBSecurityPolicy-FS-1-2-2019-08",
		"ELBSecurityPolicy-FS-1-2-Res-2019-08",
		"ELBSecurityPolicy-FS-1-2-Res-2020-10",
		"ELBSecurityPolicy-TLS13-1-2-2021-06",
		"ELBSecurityPolicy-TLS13-1-2-Res-2021-06",
		"ELBSecurityPolicy-TLS13-1-2-Ext1-2021-06",
		"ELBSecurityPolicy-TLS13-1-2-Ext2-2021-06",
		"ELBSecurityPolicy-TLS13-1-1-2021-06",
		"ELBSecurityPolicy-TLS13-1-0-2021-06",
		"ELBSecurityPolicy-TLS13-1-3-2021-06",
	}
)

func newConfigProvider(ctx context.Context, debug, disableInstrumentedHttpClient bool) (*aws.Config, error) {
	optFns := []func(*config.LoadOptions) error{
		config.WithRetryMaxAttempts(3),
	}
	if debug {
		optFns = append(optFns, config.WithClientLogMode(aws.LogRequest))
	}

	cfg, err := config.LoadDefaultConfig(ctx, optFns...)
	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config, %v", err)
	}
	if !disableInstrumentedHttpClient {
		if cfg.HTTPClient != nil {
			httpClient, ok := cfg.HTTPClient.(*http.Client)
			if !ok {
				return nil, fmt.Errorf("cfg.HTTPClient is not of type *http.Client %v", cfg.HTTPClient)
			}
			cfg.HTTPClient = instrumented_http.NewClient(httpClient, nil)
		} else {
			cfg.HTTPClient = instrumented_http.NewClient(nil, nil)
		}
	}
	return &cfg, nil
}

// NewAdapter returns a new Adapter that can be used to orchestrate and obtain information from Amazon Web Services.
// Before returning there is a discovery process for VPC and EC2 details. It tries to find the Auto Scaling Group and
// Security Group that should be used for newly created Load Balancers. If any of those critical steps fail
// an appropriate error is returned.
func NewAdapter(ctx context.Context, clusterID, newControllerID, vpcID string, debug, disableInstrumentedHttpClient bool) (adapter *Adapter, err error) {
	cfgptr, err := newConfigProvider(ctx, debug, disableInstrumentedHttpClient)
	if err != nil {
		return nil, err
	}
	cfg := *cfgptr
	adapter = &Adapter{
		ec2:                        ec2.NewFromConfig(cfg),
		elbv2:                      elbv2.NewFromConfig(cfg),
		ec2metadata:                imds.NewFromConfig(cfg),
		autoscaling:                autoscaling.NewFromConfig(cfg),
		acm:                        acm.NewFromConfig(cfg),
		iam:                        iam.NewFromConfig(cfg),
		cloudformation:             cloudformation.NewFromConfig(cfg),
		healthCheckPath:            DefaultHealthCheckPath,
		healthCheckPort:            DefaultHealthCheckPort,
		targetPort:                 DefaultTargetPort,
		healthCheckInterval:        DefaultHealthCheckInterval,
		healthCheckTimeout:         DefaultHealthCheckTimeout,
		albHealthyThresholdCount:   DefaultAlbHealthyThresholdCount,
		albUnhealthyThresholdCount: DefaultAlbUnhealthyThresholdCount,
		nlbHealthyThresholdCount:   DefaultNlbHealthyThresholdCount,
		creationTimeout:            DefaultCreationTimeout,
		ec2Details:                 make(map[string]*instanceDetails),
		singleInstances:            make(map[string]*instanceDetails),
		obsoleteInstances:          make([]string, 0),
		controllerID:               newControllerID,
		sslPolicy:                  DefaultSslPolicy,
		ipAddressType:              DefaultIpAddressType,
		albLogsS3Bucket:            DefaultAlbS3LogsBucket,
		albLogsS3Prefix:            DefaultAlbS3LogsPrefix,
		nlbCrossZone:               DefaultNLBCrossZone,
		nlbZoneAffinity:            DefaultZoneAffinity,
		nlbHTTPEnabled:             DefaultNLBHTTPEnabled,
		customFilter:               DefaultCustomFilter,
		TargetCNI: &TargetCNIconfig{
			Enabled:       false,
			TargetGroupCh: make(chan []string, 10),
		},
	}

	adapter.manifest, err = buildManifest(ctx, adapter, clusterID, vpcID)
	if err != nil {
		return nil, err
	}

	return
}

// UpdateManifest generates a manifest again based on cluster and VPC information.
// This method is useful when using custom AWS clients.
func (a *Adapter) UpdateManifest(ctx context.Context, clusterID, vpcID string) (*Adapter, error) {
	m, err := buildManifest(ctx, a, clusterID, vpcID)
	if err != nil {
		return nil, err
	}
	a.manifest = m
	return a, err
}

func (a *Adapter) NewACMCertificateProvider(certFilterTag string) certs.CertificatesProvider {
	return newACMCertProvider(a.acm, certFilterTag)
}

func (a *Adapter) NewIAMCertificateProvider(certFilterTag string) certs.CertificatesProvider {
	return newIAMCertProvider(a.iam, certFilterTag)
}

// WithHealthCheckPath returns the receiver adapter after changing the health check path that will be used by
// the resources created by the adapter.
func (a *Adapter) WithHealthCheckPath(path string) *Adapter {
	a.healthCheckPath = path
	return a
}

// WithHealthCheckPort returns the receiver adapter after changing the health check port that will be used by
// the resources created by the adapter
func (a *Adapter) WithHealthCheckPort(port uint) *Adapter {
	a.healthCheckPort = port
	return a
}

// WithAlbHealthyThresholdCount returns the receiver adapter after changing the healthy threshold count that will be used by
// the resources created by the adapter
func (a *Adapter) WithAlbHealthyThresholdCount(count uint) *Adapter {
	a.albHealthyThresholdCount = count
	return a
}

// WithAlbUnhealthyThresholdCount returns the receiver adapter after changing the unhealthy threshold count that will be used by
// the resources created by the adapter
func (a *Adapter) WithAlbUnhealthyThresholdCount(count uint) *Adapter {
	a.albUnhealthyThresholdCount = count
	return a
}

// WithNlbHealthyThresholdCount returns the receiver adapter after changing the healthy threshold count that will be used by
// the resources created by the adapter
func (a *Adapter) WithNlbHealthyThresholdCount(count uint) *Adapter {
	a.nlbHealthyThresholdCount = count
	return a
}

// WithTargetPort returns the receiver adapter after changing the target port that will be used by
// the resources created by the adapter
func (a *Adapter) WithTargetPort(port uint) *Adapter {
	a.targetPort = port
	return a
}

// WithALBHTTPTargetPort configures the ALB HTTP target port
func (a *Adapter) WithALBHTTPTargetPort(port uint) *Adapter {
	a.albHTTPTargetPort = port
	return a
}

// WithNLBHTTPTargetPort configures the NLB HTTP target port
func (a *Adapter) WithNLBHTTPTargetPort(port uint) *Adapter {
	a.nlbHTTPTargetPort = port
	return a
}

// WithTargetHTTPS returns the receiver adapter after specifying the target port will use HTTPS
func (a *Adapter) WithTargetHTTPS(https bool) *Adapter {
	a.targetHTTPS = https
	return a
}

// WithHealthCheckInterval returns the receiver adapter after changing the health check interval that will be used by
// the resources created by the adapter
func (a *Adapter) WithHealthCheckInterval(interval time.Duration) *Adapter {
	a.healthCheckInterval = interval
	return a
}

// WithHealthCheckTimeout returns the receiver adapter after changing the health check timeout that will be used by
// the resources created by the adapter
func (a *Adapter) WithHealthCheckTimeout(timeout time.Duration) *Adapter {
	a.healthCheckTimeout = timeout
	return a
}

// WithCreationTimeout returns the receiver adapter after changing the creation timeout that is used as the max wait
// time for the creation of all the required AWS resources for a given Ingress
func (a *Adapter) WithCreationTimeout(interval time.Duration) *Adapter {
	a.creationTimeout = interval
	return a
}

// WithIdleConnectionTimeout returns the receiver adapter after
// changing the idle connection timeout that is used to change the
// corresponding LoadBalancerAttributes in the CloudFormation stack
// that creates the LoadBalancers.
// https://docs.aws.amazon.com/elasticloadbalancing/latest/application/application-load-balancers.html#connection-idle-timeout
func (a *Adapter) WithIdleConnectionTimeout(interval time.Duration) *Adapter {
	if 1*time.Second <= interval && interval <= 4000*time.Second {
		a.idleConnectionTimeout = interval
	}
	return a
}

// WithDeregistrationDelayTimeout returns the receiver adapter after
// changing the deregistration delay timeout for the managed target
// groups.
// https://docs.aws.amazon.com/elasticloadbalancing/latest/network/load-balancer-target-groups.html#deregistration-delay
func (a *Adapter) WithDeregistrationDelayTimeout(interval time.Duration) *Adapter {
	if 1*time.Second <= interval && interval <= 3600*time.Second {
		a.deregistrationDelayTimeout = interval
	}
	return a
}

// WithControllerID returns the receiver adapter after changing the CloudFormation template that should be used
// to create Load Balancer stacks
func (a *Adapter) WithControllerID(id string) *Adapter {
	a.controllerID = id
	return a
}

// WithSslPolicy returns the receiver adapter after changing the CloudFormation template that should be used
// to create Load Balancer stacks
func (a *Adapter) WithSslPolicy(policy string) *Adapter {
	a.sslPolicy = policy
	return a
}

// WithStackTerminationProtection returns the receiver adapter after changing
// the stack termination protection value.
func (a *Adapter) WithStackTerminationProtection(terminationProtection bool) *Adapter {
	a.stackTerminationProtection = terminationProtection
	return a
}

// WithStackTags returns the receiver adapter after setting the stackTags
// value.
func (a *Adapter) WithStackTags(tags map[string]string) *Adapter {
	a.stackTags = tags
	return a
}

// WithIpAddressType returns the receiver with ipv4 or dualstack configuration, defaults to ipv4.
func (a *Adapter) WithIpAddressType(ipAddressType string) *Adapter {
	if ipAddressType == IPAddressTypeDualstack {
		a.ipAddressType = ipAddressType
	}
	return a
}

// WithAlbLogsS3Bucket returns the receiver adapter after changing the S3 bucket for logging
func (a *Adapter) WithAlbLogsS3Bucket(bucket string) *Adapter {
	a.albLogsS3Bucket = bucket
	return a
}

// WithAlbLogsS3Bucket returns the receiver adapter after changing the S3 prefix within the bucket for logging
func (a *Adapter) WithAlbLogsS3Prefix(prefix string) *Adapter {
	a.albLogsS3Prefix = prefix
	return a
}

// WithHTTPRedirectToHTTPS returns the receiver adapter after changing the flag to effect HTTP->HTTPS redirection
func (a *Adapter) WithHTTPRedirectToHTTPS(httpRedirectToHTTPS bool) *Adapter {
	a.httpRedirectToHTTPS = httpRedirectToHTTPS
	return a
}

// WithNLBCrossZone returns the receiver adapter after setting the nlbCrossZone
// config.
func (a *Adapter) WithNLBCrossZone(nlbCrossZone bool) *Adapter {
	a.nlbCrossZone = nlbCrossZone
	return a
}

// WithNLBZoneAffinity returns the receiver adapter after setting the
// nlbZoneAffinity config.
func (a *Adapter) WithNLBZoneAffinity(nlbZoneAffinity string) *Adapter {
	a.nlbZoneAffinity = nlbZoneAffinity
	return a
}

// WithNLBHTTPEnabled returns the receiver adapter after setting the
// nlbHTTPEnabled config.
func (a *Adapter) WithNLBHTTPEnabled(nlbHTTPEnabled bool) *Adapter {
	a.nlbHTTPEnabled = nlbHTTPEnabled
	return a
}

// WithCustomFilter returns the receiver adapter after setting a custom filter expression
func (a *Adapter) WithCustomFilter(customFilter string) *Adapter {
	a.customFilter = customFilter
	// also rebuild related manifest items
	a.manifest.filters = a.parseFilters(a.ClusterID())
	a.manifest.asgFilters = a.parseAutoscaleFilterTags(a.ClusterID())
	return a
}

// WithInternalDomains returns the receiver adapter after changing the
// internal domains that will be used by the resources created by the
// adapter.
func (a *Adapter) WithInternalDomains(domains []string) *Adapter {
	a.internalDomains = domains
	return a
}

// WithTargetAccessMode returns the receiver adapter after defining the target access mode
func (a *Adapter) WithTargetAccessMode(mode string) *Adapter {
	a.TargetCNI.Enabled = mode == TargetAccessModeAWSCNI

	switch mode {
	case TargetAccessModeHostPort:
		a.targetType = elbv2Types.TargetTypeEnumInstance
	case TargetAccessModeAWSCNI:
		a.targetType = elbv2Types.TargetTypeEnumIp
	case TargetAccessModeLegacy:
		a.targetType = ""
	}
	return a
}

// WithDenyInternalDomains returns the receiver adapter after setting
// the denyInternalDomains config.
func (a *Adapter) WithDenyInternalDomains(deny bool) *Adapter {
	a.denyInternalDomains = deny
	return a
}

// WithInternalDomainsDenyResponse returns the receiver adapter after
// changing the Body of the response that will be returned by the
// resources created by the adapter when denyInternalDomains is set to
// true.
func (a *Adapter) WithInternalDomainsDenyResponse(body string) *Adapter {
	a.denyInternalRespBody = body
	return a
}

// WithInternalDomainsDenyResponseStatusCode returns the receiver
// adapter after changing the status code of the response that will be
// returned by the resources created by the adapter when
// denyInternalDomains is set to true.
func (a *Adapter) WithInternalDomainsDenyResponseStatusCode(code int) *Adapter {
	a.denyInternalRespStatusCode = code
	return a
}

// WithInternalDomainsDenyResponseContenType returns the receiver
// adapter after changing the content-type of the response that will be
// returned by the resources created by the adapter when
// denyInternalDomains is set to true.
func (a *Adapter) WithInternalDomainsDenyResponseContenType(contentType string) *Adapter {
	a.denyInternalRespContentType = contentType
	return a
}

// WithCustomEc2Client returns an Adapter that will use the provided
// EC2 client, instead of the EC2 client provided by AWS.
func (a *Adapter) WithCustomEc2Client(c EC2API) *Adapter {
	a.ec2 = c
	return a
}

// WithCustomAutoScalingClient returns an Adapter that will use the provided
// Auto scaling client, instead of the Auto scaling client provided by AWS.
func (a *Adapter) WithCustomAutoScalingClient(c AutoScalingAPI) *Adapter {
	a.autoscaling = c
	return a
}

// WithCustomElbv2Client returns an Adapter that will use the provided
// ELBv2 client, instead of the ELBv2 client provided by AWS.
func (a *Adapter) WithCustomElbv2Client(c ELBV2API) *Adapter {
	a.elbv2 = c
	return a
}

// WithCustomCloudFormationClient returns an Adapter that will use the provided
// CloudFormation client, instead of the CloudFormation client provided by AWS.
func (a *Adapter) WithCustomCloudFormationClient(c CloudFormationAPI) *Adapter {
	a.cloudformation = c
	return a
}

// ClusterID returns the ClusterID tag that all resources from the same Kubernetes cluster share.
// It's taken from the current ec2 instance.
func (a *Adapter) ClusterID() string {
	return a.manifest.clusterID
}

// VpcID returns the VPC ID the current node belongs to.
func (a *Adapter) VpcID() string {
	return a.manifest.vpcID
}

// InstanceID returns the instance ID the current node is running on.
func (a *Adapter) InstanceID() string {
	if a.manifest != nil && a.manifest.instance != nil {
		return a.manifest.instance.id
	}
	return "<none>"
}

// S3Bucket returns the S3 Bucket to be logged to
func (a *Adapter) S3Bucket() string {
	return a.albLogsS3Bucket
}

// S3Prefix returns the S3 Prefix to be logged to
func (a *Adapter) S3Prefix() string {
	return a.albLogsS3Prefix
}

// SingleInstances returns list of IDs of instances that do not belong to any
// Auto Scaling Group and should be managed manually.
func (a *Adapter) SingleInstances() []string {
	instances := make([]string, 0, len(a.singleInstances))
	for id := range a.singleInstances {
		instances = append(instances, id)
	}
	return instances
}

// RunningSingleInstances returns list of IDs of running instances that do
// not belong to any Auto Scaling Group and should be managed manually.
func (a Adapter) RunningSingleInstances() []string {
	instances := make([]string, 0, len(a.singleInstances))
	for id, details := range a.singleInstances {
		if details.running {
			instances = append(instances, id)
		}
	}
	return instances
}

// ObsoleteSingleInstances returns list of IDs of instances that should be deregistered
// from all Target Groups.
func (a Adapter) ObsoleteSingleInstances() []string {
	return a.obsoleteInstances
}

// Get number of instances in cache.
func (a Adapter) CachedInstances() int {
	return len(a.ec2Details)
}

// Get EC2 filters that are used to filter instances that are loaded using DescribeInstances.
func (a Adapter) FiltersString() string {
	result := ""
	for _, filter := range a.manifest.filters {
		result += fmt.Sprintf("%s=%s ", aws.ToString(filter.Name), strings.Join(filter.Values, ","))
	}
	return strings.TrimSpace(result)
}

// SecurityGroupID returns the security group ID that should be used to create Load Balancers.
func (a *Adapter) SecurityGroupID() string {
	return a.manifest.securityGroup.id
}

// FindManagedStacks returns all CloudFormation stacks containing the controller management tags
// that match the current cluster and are ready to be used. The stack status is used to filter.
func (a *Adapter) FindManagedStacks(ctx context.Context) ([]*Stack, error) {
	stacks, err := findManagedStacks(ctx, a.cloudformation, a.ClusterID(), a.controllerID)
	if err != nil {
		return nil, err
	}
	return stacks, nil
}

// UpdateTargetGroupsAndAutoScalingGroups updates Auto Scaling Groups
// config to have relevant Target Groups and registers/deregisters single
// instances (that do not belong to ASG) in relevant Target Groups.
func (a *Adapter) UpdateTargetGroupsAndAutoScalingGroups(ctx context.Context, stacks []*Stack, problems *problem.List) {
	allTargetGroupARNs := make([]string, 0, len(stacks))
	for _, stack := range stacks {
		if len(stack.TargetGroupARNs) > 0 {
			allTargetGroupARNs = append(allTargetGroupARNs, stack.TargetGroupARNs...)
		}
	}
	// split the full list into TG types
	targetTypesARNs, err := categorizeTargetTypeInstance(ctx, a.elbv2, allTargetGroupARNs)
	if err != nil {
		problems.Add("failed to categorize Target Type Instance: %w", err)
		return
	}

	// update the CNI TG list
	if a.TargetCNI.Enabled {
		a.TargetCNI.TargetGroupCh <- targetTypesARNs[elbv2Types.TargetTypeEnumIp]
	}

	// remove the IP TGs from the list keeping all other TGs including problematic #127 and nonexistent #436
	targetGroupARNs := difference(allTargetGroupARNs, targetTypesARNs[elbv2Types.TargetTypeEnumIp])

	ownerTags := map[string]string{
		clusterIDTagPrefix + a.ClusterID(): resourceLifecycleOwned,
		kubernetesCreatorTag:               a.controllerID,
	}

	for _, asg := range a.TargetedAutoScalingGroups {
		// This call is idempotent and safe to execute every time
		if err := updateTargetGroupsForAutoScalingGroup(ctx, a.autoscaling, a.elbv2, targetGroupARNs, asg.name, ownerTags); err != nil {
			problems.Add("failed to update target groups for autoscaling group %q: %w", asg.name, err)
		}
	}

	// remove owned TGs from non-targeted ASGs
	nonTargetedASGs := nonTargetedASGs(a.OwnedAutoScalingGroups, a.TargetedAutoScalingGroups)
	for _, asg := range nonTargetedASGs {
		// This call is idempotent and safe to execute every time
		if err := updateTargetGroupsForAutoScalingGroup(ctx, a.autoscaling, a.elbv2, nil, asg.name, ownerTags); err != nil {
			problems.Add("failed to update target groups for non-targeted autoscaling group %q: %w", asg.name, err)
		}
	}

	runningSingleInstances := a.RunningSingleInstances()
	if len(runningSingleInstances) != 0 {
		// This call is idempotent too
		if err := registerTargetsOnTargetGroups(ctx, a.elbv2, targetGroupARNs, runningSingleInstances); err != nil {
			problems.Add("failed to register instances %q in target groups: %w", runningSingleInstances, err)
		}
	}
	if len(a.obsoleteInstances) != 0 {
		// Deregister instances from target groups and clean up list of obsolete instances
		if err := deregisterTargetsOnTargetGroups(ctx, a.elbv2, targetGroupARNs, a.obsoleteInstances); err != nil {
			problems.Add("failed to deregister instances %q in target groups: %w", a.obsoleteInstances, err)
		} else {
			a.obsoleteInstances = make([]string, 0)
		}
	}
}

// CreateStack creates a new Application Load Balancer using CloudFormation.
// The stack name is derived from the Cluster ID and a has of the certificate
// ARNs (when available).
// All the required resources (listeners and target group) are created in a
// transactional fashion.
// Failure to create the stack causes it to be deleted automatically.
func (a *Adapter) CreateStack(ctx context.Context, certificateARNs []string, scheme, securityGroup, owner, sslPolicy, ipAddressType, wafWebACLID string, cwAlarms CloudWatchAlarmList, loadBalancerType string, http2 bool) (string, error) {
	certARNs := make(map[string]time.Time, len(certificateARNs))
	for _, arn := range certificateARNs {
		certARNs[arn] = time.Time{}
	}

	if sslPolicy == "" {
		sslPolicy = a.sslPolicy
	}

	if _, ok := SSLPolicies[sslPolicy]; !ok {
		return "", fmt.Errorf("invalid SSLPolicy '%s' defined", sslPolicy)
	}

	spec := &stackSpec{
		name:            a.stackName(),
		scheme:          scheme,
		ownerIngress:    owner,
		certificateARNs: certARNs,
		securityGroupID: securityGroup,
		subnets:         a.FindLBSubnets(scheme),
		vpcID:           a.VpcID(),
		clusterID:       a.ClusterID(),
		healthCheck: &healthCheck{
			path:     a.healthCheckPath,
			port:     a.healthCheckPort,
			interval: a.healthCheckInterval,
			timeout:  a.healthCheckTimeout,
		},
		albHealthyThresholdCount:          a.albHealthyThresholdCount,
		albUnhealthyThresholdCount:        a.albUnhealthyThresholdCount,
		nlbHealthyThresholdCount:          a.nlbHealthyThresholdCount,
		targetType:                        a.targetType,
		targetPort:                        a.targetPort,
		targetHTTPS:                       a.targetHTTPS,
		httpDisabled:                      a.httpDisabled(loadBalancerType),
		httpTargetPort:                    a.httpTargetPort(loadBalancerType),
		timeoutInMinutes:                  int32(a.creationTimeout.Minutes()),
		stackTerminationProtection:        a.stackTerminationProtection,
		idleConnectionTimeoutSeconds:      uint(a.idleConnectionTimeout.Seconds()),
		deregistrationDelayTimeoutSeconds: uint(a.deregistrationDelayTimeout.Seconds()),
		controllerID:                      a.controllerID,
		sslPolicy:                         sslPolicy,
		ipAddressType:                     ipAddressType,
		loadbalancerType:                  loadBalancerType,
		albLogsS3Bucket:                   a.albLogsS3Bucket,
		albLogsS3Prefix:                   a.albLogsS3Prefix,
		wafWebAclId:                       wafWebACLID,
		cwAlarms:                          cwAlarms,
		httpRedirectToHTTPS:               a.httpRedirectToHTTPS,
		nlbCrossZone:                      a.nlbCrossZone,
		nlbZoneAffinity:                   a.nlbZoneAffinity,
		http2:                             http2,
		tags:                              a.stackTags,
		internalDomains:                   a.internalDomains,
		denyInternalDomains:               a.denyInternalDomains,
		denyInternalDomainsResponse: denyResp{
			body:        a.denyInternalRespBody,
			statusCode:  a.denyInternalRespStatusCode,
			contentType: a.denyInternalRespContentType,
		},
	}

	return createStack(ctx, a.cloudformation, spec)
}

func (a *Adapter) UpdateStack(ctx context.Context, stackName string, certificateARNs map[string]time.Time, scheme, securityGroup, owner, sslPolicy, ipAddressType, wafWebACLID string, cwAlarms CloudWatchAlarmList, loadBalancerType string, http2 bool) (string, error) {
	if _, ok := SSLPolicies[sslPolicy]; !ok {
		return "", fmt.Errorf("invalid SSLPolicy '%s' defined", sslPolicy)
	}

	spec := &stackSpec{
		name:            stackName,
		scheme:          scheme,
		ownerIngress:    owner,
		certificateARNs: certificateARNs,
		securityGroupID: securityGroup,
		subnets:         a.FindLBSubnets(scheme),
		vpcID:           a.VpcID(),
		clusterID:       a.ClusterID(),
		healthCheck: &healthCheck{
			path:     a.healthCheckPath,
			port:     a.healthCheckPort,
			interval: a.healthCheckInterval,
			timeout:  a.healthCheckTimeout,
		},
		albHealthyThresholdCount:          a.albHealthyThresholdCount,
		albUnhealthyThresholdCount:        a.albUnhealthyThresholdCount,
		nlbHealthyThresholdCount:          a.nlbHealthyThresholdCount,
		targetType:                        a.targetType,
		targetPort:                        a.targetPort,
		targetHTTPS:                       a.targetHTTPS,
		httpDisabled:                      a.httpDisabled(loadBalancerType),
		httpTargetPort:                    a.httpTargetPort(loadBalancerType),
		timeoutInMinutes:                  int32(a.creationTimeout.Minutes()),
		stackTerminationProtection:        a.stackTerminationProtection,
		idleConnectionTimeoutSeconds:      uint(a.idleConnectionTimeout.Seconds()),
		deregistrationDelayTimeoutSeconds: uint(a.deregistrationDelayTimeout.Seconds()),
		controllerID:                      a.controllerID,
		sslPolicy:                         sslPolicy,
		ipAddressType:                     ipAddressType,
		loadbalancerType:                  loadBalancerType,
		albLogsS3Bucket:                   a.albLogsS3Bucket,
		albLogsS3Prefix:                   a.albLogsS3Prefix,
		wafWebAclId:                       wafWebACLID,
		cwAlarms:                          cwAlarms,
		httpRedirectToHTTPS:               a.httpRedirectToHTTPS,
		nlbCrossZone:                      a.nlbCrossZone,
		nlbZoneAffinity:                   a.nlbZoneAffinity,
		http2:                             http2,
		tags:                              a.stackTags,
		internalDomains:                   a.internalDomains,
		denyInternalDomains:               a.denyInternalDomains,
		denyInternalDomainsResponse: denyResp{
			body:        a.denyInternalRespBody,
			statusCode:  a.denyInternalRespStatusCode,
			contentType: a.denyInternalRespContentType,
		},
	}

	return updateStack(ctx, a.cloudformation, spec)
}

func (a *Adapter) GetLoadBalancer(ctx context.Context, stackName string) (*elbv2Types.LoadBalancer, error) {
	lbStackResources, err := getLoadBalancerStackResource(ctx, a.cloudformation, stackName)
	if err != nil {
		return nil, fmt.Errorf("failed to get load balancer stack resource: %w", err)
	}

	loadBalancerID := aws.ToString(lbStackResources.PhysicalResourceId)
	lbs, err := a.elbv2.DescribeLoadBalancers(ctx, &elbv2.DescribeLoadBalancersInput{
		LoadBalancerArns: []string{loadBalancerID},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to describe load balancer %q: %w", loadBalancerID, err)
	}
	if len(lbs.LoadBalancers) == 0 {
		return nil, fmt.Errorf("load balancer %q not found", loadBalancerID)
	}
	if len(lbs.LoadBalancers) > 1 {
		return nil, fmt.Errorf("multiple load balancers found for physical-Id %q", loadBalancerID)
	}
	return &lbs.LoadBalancers[0], nil
}

func (a *Adapter) httpTargetPort(loadBalancerType string) uint {
	if loadBalancerType == LoadBalancerTypeApplication && a.albHTTPTargetPort != 0 {
		return a.albHTTPTargetPort
	} else if loadBalancerType == LoadBalancerTypeNetwork && a.nlbHTTPTargetPort != 0 {
		return a.nlbHTTPTargetPort
	}
	return a.targetPort
}

func (a *Adapter) httpDisabled(loadBalancerType string) bool {
	if loadBalancerType == LoadBalancerTypeNetwork {
		return !a.nlbHTTPEnabled
	}
	return false
}

func (a *Adapter) stackName() string {
	return normalizeStackName(a.ClusterID())
}

// GetStack returns the CloudFormation stack details with the name or ID from the argument
func (a *Adapter) GetStack(ctx context.Context, stackID string) (*Stack, error) {
	return getStack(ctx, a.cloudformation, stackID)
}

// DeleteStack deletes the CloudFormation stack with the given name
func (a *Adapter) DeleteStack(ctx context.Context, stack *Stack) error {
	for _, asg := range a.TargetedAutoScalingGroups {
		if err := detachTargetGroupsFromAutoScalingGroup(ctx, a.autoscaling, stack.TargetGroupARNs, asg.name); err != nil {
			return fmt.Errorf("failed to detach target groups from autoscaling group %q: %w", asg.name, err)
		}
	}
	return deleteStack(ctx, a.cloudformation, stack.Name)
}

func buildManifest(ctx context.Context, awsAdapter *Adapter, clusterID, vpcID string) (*manifest, error) {
	var err error
	var instanceDetails *instanceDetails

	if clusterID == "" || vpcID == "" {
		log.Debug("aws.ec2metadata.GetMetadata")
		myID, err := awsAdapter.ec2metadata.GetMetadata(ctx, &imds.GetMetadataInput{
			Path: "instance-id",
		})
		if err != nil {
			return nil, err
		}

		log.Debug("aws.getInstanceDetails")
		content, err := io.ReadAll(myID.Content)
		if err != nil {
			return nil, fmt.Errorf("failed to read metadata content: %w", err)
		}
		defer myID.Content.Close()
		instanceDetails, err = getInstanceDetails(ctx, awsAdapter.ec2, string(content))
		if err != nil {
			return nil, err
		}
		clusterID = instanceDetails.clusterID()
		vpcID = instanceDetails.vpcID
	}

	log.Debug("aws.findSecurityGroupWithClusterID")
	securityGroupDetails, err := findSecurityGroupWithClusterID(ctx, awsAdapter.ec2, clusterID, awsAdapter.controllerID)
	if err != nil {
		return nil, err
	}

	log.Debug("aws.getSubnets")
	subnets, err := getSubnets(ctx, awsAdapter.ec2, vpcID, clusterID)
	if err != nil {
		return nil, err
	}
	if len(subnets) == 0 {
		return nil, ErrNoSubnets
	}

	return &manifest{
		securityGroup: securityGroupDetails,
		instance:      instanceDetails,
		subnets:       subnets,
		filters:       awsAdapter.parseFilters(clusterID),
		asgFilters:    awsAdapter.parseAutoscaleFilterTags(clusterID),
		clusterID:     clusterID,
		vpcID:         vpcID,
	}, nil
}

// FindLBSubnets finds subnets for an ALB based on the scheme.
//
// It follows the same logic for finding subnets as the kube-controller-manager
// when finding subnets for ELBs used for services of type LoadBalancer.
// https://github.com/kubernetes/kubernetes/blob/65efeee64f772e0f38037e91a677138a335a7570/pkg/cloudprovider/providers/aws/aws.go#L2949-L3027
func (a *Adapter) FindLBSubnets(scheme string) []string {
	var internal bool
	if scheme == string(elbv2Types.LoadBalancerSchemeEnumInternal) {
		internal = true
	}

	subnetsByAZ := make(map[string]*subnetDetails)
	for _, subnet := range a.manifest.subnets {
		// ignore private subnet for public LB
		if !internal && !subnet.public {
			continue
		}

		existing, ok := subnetsByAZ[subnet.availabilityZone]
		if !ok {
			subnetsByAZ[subnet.availabilityZone] = subnet
			continue
		}

		// prefer subnet with an elb role tag
		var tagName string
		if internal {
			tagName = internalELBRoleTagName
		} else {
			tagName = elbRoleTagName
		}

		_, existingHasTag := existing.tags[tagName]
		_, subnetHasTag := subnet.tags[tagName]

		if existingHasTag != subnetHasTag {
			if subnetHasTag {
				subnetsByAZ[subnet.availabilityZone] = subnet
			}
			continue
		}

		// If we have two subnets for the same AZ we arbitrarily choose
		// the one that is first lexicographically.
		if strings.Compare(existing.id, subnet.id) > 0 {
			subnetsByAZ[subnet.availabilityZone] = subnet
		}
	}

	subnetIDs := make([]string, 0, len(subnetsByAZ))
	for _, subnet := range subnetsByAZ {
		subnetIDs = append(subnetIDs, subnet.id)
	}

	return subnetIDs
}

func getNameTag(tags map[string]string) (string, error) {
	if name, err := getTag(tags, nameTag); err == nil {
		return name, nil
	}
	return "<no name tag>", ErrMissingNameTag
}

func getTag(tags map[string]string, tagName string) (string, error) {
	if name, has := tags[tagName]; has {
		return name, nil
	}
	return "<missing tag>", ErrMissingTag
}

// UpdateAutoScalingGroupsAndInstances updates list of known ASGs and EC2 instances.
func (a *Adapter) UpdateAutoScalingGroupsAndInstances(ctx context.Context) error {
	var err error

	a.ec2Details, err = getInstancesDetailsWithFilters(ctx, a.ec2, a.manifest.filters)
	if err != nil {
		return err
	}

	newSingleInstances := make(map[string]*instanceDetails)
	for instanceID, details := range a.singleInstances {
		if _, ok := a.ec2Details[instanceID]; !ok {
			// Instance does not exist on EC2 anymore, add it to list of obsolete instances
			a.obsoleteInstances = append(a.obsoleteInstances, instanceID)
		} else {
			// Instance exists, so keep it in the list of single instances
			newSingleInstances[instanceID] = details
		}
	}
	a.singleInstances = newSingleInstances

	for instanceID, details := range a.ec2Details {
		_, err := getAutoScalingGroupName(details.tags)
		if err != nil {
			// Instance is not in ASG, save in single instances list.
			a.singleInstances[instanceID] = details
			continue
		}
	}

	ownedTag := map[string]string{
		clusterIDTagPrefix + a.ClusterID(): resourceLifecycleOwned,
	}

	targetedASGs, ownedASGs, err := getOwnedAndTargetedAutoScalingGroups(ctx, a.autoscaling, a.manifest.asgFilters, ownedTag)
	if err != nil {
		return err
	}

	a.TargetedAutoScalingGroups = targetedASGs
	a.OwnedAutoScalingGroups = ownedASGs
	return nil
}

// Create EC2 filter that will be used to filter instances when calling DescribeInstances
// later on each cycle. Filter is based on value of customTagFilterEnvVarName environment
// veriable. If it is undefined or could not be parsed, default filter is returned which
// filters on kubernetesClusterTag tag value and kubernetesNodeRoleTag existance.
func (a *Adapter) parseFilters(clusterId string) []ec2Types.Filter {
	if a.customFilter != "" {
		terms := strings.Fields(a.customFilter)
		filters := make([]ec2Types.Filter, len(terms))
		for i, term := range terms {
			parts := strings.Split(term, "=")
			if len(parts) != 2 {
				log.Errorf("Failed parsing %s, falling back to default", a.customFilter)
				return generateDefaultFilters(clusterId)
			}
			filters[i] = ec2Types.Filter{
				Name:   aws.String(parts[0]),
				Values: strings.Split(parts[1], ","),
			}
		}
		return filters
	}
	return generateDefaultFilters(clusterId)
}

// Generate default EC2 filter for usage with ECs DescribeInstances call based on EC2 tags
// of instance where Ingress Controller pod was started.
func generateDefaultFilters(clusterId string) []ec2Types.Filter {
	return []ec2Types.Filter{
		{
			Name:   aws.String("tag:" + clusterIDTagPrefix + clusterId),
			Values: []string{resourceLifecycleOwned},
		},
		{
			Name:   aws.String("tag-key"),
			Values: []string{kubernetesNodeRoleTag},
		},
	}
}

// We look to the same string used for instance filtering, however, we are much more limited in what can be done for
// ASGs. As such, we instead build a map of tags to look for as we iterate over all ASGs in getOwnedAutoScalingGroups
func (a *Adapter) parseAutoscaleFilterTags(clusterId string) map[string][]string {
	if a.customFilter != "" {
		terms := strings.Fields(a.customFilter)
		filterTags := make(map[string][]string)
		for _, term := range terms {
			parts := strings.Split(term, "=")
			if len(parts) != 2 {
				log.Errorf("Failed parsing %s, falling back to default", a.customFilter)
				return generateDefaultAutoscaleFilterTags(clusterId)
			}
			if parts[0] == "tag-key" {
				filterTags[parts[1]] = []string{}
			} else if strings.HasPrefix(parts[0], "tag:") {
				tagKey := strings.TrimPrefix(parts[0], "tag:")
				filterTags[tagKey] = strings.Split(parts[1], ",")
			} else {
				filterTags[parts[0]] = strings.Split(parts[1], ",")
			}
		}
		return filterTags
	}
	return generateDefaultAutoscaleFilterTags(clusterId)

}

func generateDefaultAutoscaleFilterTags(clusterId string) map[string][]string {
	filterTags := make(map[string][]string)
	filterTags[clusterIDTagPrefix+clusterId] = []string{resourceLifecycleOwned}
	return filterTags
}

func nonTargetedASGs(ownedASGs, targetedASGs map[string]*autoScalingGroupDetails) map[string]*autoScalingGroupDetails {
	nonTargetedASGs := make(map[string]*autoScalingGroupDetails)

	for name, asg := range ownedASGs {
		if _, ok := targetedASGs[name]; !ok {
			nonTargetedASGs[name] = asg
		}
	}

	return nonTargetedASGs
}

// SetTargetsOnCNITargetGroups implements desired state for CNI target groups
// by polling the current list of targets thus creating a diff of what needs to be added and removed.
func (a *Adapter) SetTargetsOnCNITargetGroups(ctx context.Context, endpoints, cniTargetGroupARNs []string) error {
	log.Debugf("setting targets on CNI target groups: '%v'", cniTargetGroupARNs)
	for _, targetGroupARN := range cniTargetGroupARNs {
		tgh, err := a.elbv2.DescribeTargetHealth(ctx, &elbv2.DescribeTargetHealthInput{TargetGroupArn: &targetGroupARN})
		if err != nil {
			log.Errorf("unable to describe target health %v", err)
			// continue for processing of the rest of the target groups
			continue
		}
		registeredInstances := make([]string, len(tgh.TargetHealthDescriptions))
		for i, target := range tgh.TargetHealthDescriptions {
			registeredInstances[i] = *target.Target.Id
		}
		toRegister := difference(endpoints, registeredInstances)
		if len(toRegister) > 0 {
			log.Info("Registering CNI targets: ", toRegister)
			err := registerTargetsOnTargetGroups(ctx, a.elbv2, []string{targetGroupARN}, toRegister)
			if err != nil {
				return err
			}
		}
		toDeregister := difference(registeredInstances, endpoints)
		if len(toDeregister) > 0 {
			log.Info("Deregistering CNI targets: ", toDeregister)
			err := deregisterTargetsOnTargetGroups(ctx, a.elbv2, []string{targetGroupARN}, toDeregister)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// difference returns the elements in `a` that aren't in `b`.
func difference(a, b []string) []string {
	mb := make(map[string]struct{}, len(b))
	for _, x := range b {
		mb[x] = struct{}{}
	}
	var diff []string
	for _, x := range a {
		if _, found := mb[x]; !found {
			diff = append(diff, x)
		}
	}
	return diff
}
