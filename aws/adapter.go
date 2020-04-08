package aws

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/acm"
	"github.com/aws/aws-sdk-go/service/acm/acmiface"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/aws/aws-sdk-go/service/elbv2/elbv2iface"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/linki/instrumented_http"
	log "github.com/sirupsen/logrus"
	"github.com/zalando-incubator/kube-ingress-aws-controller/certs"
)

// An Adapter can be used to orchestrate and obtain information from Amazon Web Services.
type Adapter struct {
	ec2metadata    *ec2metadata.EC2Metadata
	ec2            ec2iface.EC2API
	elbv2          elbv2iface.ELBV2API
	autoscaling    autoscalingiface.AutoScalingAPI
	acm            acmiface.ACMAPI
	iam            iamiface.IAMAPI
	cloudformation cloudformationiface.CloudFormationAPI

	manifest                   *manifest
	healthCheckPath            string
	healthCheckPort            uint
	healthCheckInterval        time.Duration
	targetPort                 uint
	creationTimeout            time.Duration
	idleConnectionTimeout      time.Duration
	stackTTL                   time.Duration
	TargetedAutoScalingGroups  map[string]*autoScalingGroupDetails
	OwnedAutoScalingGroups     map[string]*autoScalingGroupDetails
	ec2Details                 map[string]*instanceDetails
	singleInstances            map[string]*instanceDetails
	obsoleteInstances          []string
	stackTerminationProtection bool
	controllerID               string
	sslPolicy                  string
	ipAddressType              string
	albLogsS3Bucket            string
	albLogsS3Prefix            string
	wafWebAclId                string
	httpRedirectToHTTPS        bool
	nlbCrossZone               bool
	nlbHTTPEnabled             bool
	customFilter               string
}

type manifest struct {
	securityGroup *securityGroupDetails
	instance      *instanceDetails
	subnets       []*subnetDetails
	filters       []*ec2.Filter
	asgFilters    map[string][]string
}

type configProviderFunc func() client.ConfigProvider

const (
	DefaultHealthCheckPath           = "/kube-system/healthz"
	DefaultHealthCheckPort           = 9999
	DefaultTargetPort                = 9999
	DefaultHealthCheckInterval       = 10 * time.Second
	DefaultCertificateUpdateInterval = 30 * time.Minute
	DefaultCreationTimeout           = 5 * time.Minute
	DefaultStackTTL                  = 5 * time.Minute
	DefaultIdleConnectionTimeout     = 1 * time.Minute
	DefaultControllerID              = "kube-ingress-aws-controller"
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
	// DefaultWAFWebAclId is a blank string, set if desired.
	DefaultWAFWebAclId = ""

	DefaultCustomFilter = ""
	// DefaultNLBCrossZone specifies the default configuration for cross
	// zone load balancing: https://docs.aws.amazon.com/elasticloadbalancing/latest/network/network-load-balancers.html#load-balancer-attributes
	DefaultNLBCrossZone   = false
	DefaultNLBHTTPEnabled = false

	nameTag                     = "Name"
	LoadBalancerTypeApplication = "application"
	LoadBalancerTypeNetwork     = "network"
	IPAddressTypeIPV4           = "ipv4"
	IPAddressTypeDualstack      = "dualstack"
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
		"ELBSecurityPolicy-2016-08":             true,
		"ELBSecurityPolicy-FS-2018-06":          true,
		"ELBSecurityPolicy-TLS-1-2-2017-01":     true,
		"ELBSecurityPolicy-TLS-1-2-Ext-2018-06": true,
		"ELBSecurityPolicy-TLS-1-1-2017-01":     true,
		"ELBSecurityPolicy-2015-05":             true,
		"ELBSecurityPolicy-TLS-1-0-2015-04":     true,
	}
	SSLPoliciesList = []string{
		"ELBSecurityPolicy-2016-08",
		"ELBSecurityPolicy-FS-2018-06",
		"ELBSecurityPolicy-TLS-1-2-2017-01",
		"ELBSecurityPolicy-TLS-1-2-Ext-2018-06",
		"ELBSecurityPolicy-TLS-1-1-2017-01",
		"ELBSecurityPolicy-2015-05",
		"ELBSecurityPolicy-TLS-1-0-2015-04",
	}
)

var configProvider = defaultConfigProvider

func defaultConfigProvider() client.ConfigProvider {
	cfg := aws.NewConfig().WithMaxRetries(3)
	cfg = cfg.WithHTTPClient(instrumented_http.NewClient(cfg.HTTPClient, nil))
	opts := session.Options{
		SharedConfigState: session.SharedConfigEnable,
		Config:            *cfg,
	}
	return session.Must(session.NewSessionWithOptions(opts))
}

// NewAdapter returns a new Adapter that can be used to orchestrate and obtain information from Amazon Web Services.
// Before returning there is a discovery process for VPC and EC2 details. It tries to find the Auto Scaling Group and
// Security Group that should be used for newly created Load Balancers. If any of those critical steps fail
// an appropriate error is returned.
func NewAdapter(newControllerID string) (adapter *Adapter, err error) {
	p := configProvider()
	adapter = &Adapter{
		ec2:                 ec2.New(p),
		elbv2:               elbv2.New(p),
		ec2metadata:         ec2metadata.New(p),
		autoscaling:         autoscaling.New(p),
		acm:                 acm.New(p),
		iam:                 iam.New(p),
		cloudformation:      cloudformation.New(p),
		healthCheckPath:     DefaultHealthCheckPath,
		healthCheckPort:     DefaultHealthCheckPort,
		targetPort:          DefaultTargetPort,
		healthCheckInterval: DefaultHealthCheckInterval,
		creationTimeout:     DefaultCreationTimeout,
		stackTTL:            DefaultStackTTL,
		ec2Details:          make(map[string]*instanceDetails),
		singleInstances:     make(map[string]*instanceDetails),
		obsoleteInstances:   make([]string, 0),
		controllerID:        newControllerID,
		sslPolicy:           DefaultSslPolicy,
		ipAddressType:       DefaultIpAddressType,
		albLogsS3Bucket:     DefaultAlbS3LogsBucket,
		albLogsS3Prefix:     DefaultAlbS3LogsPrefix,
		wafWebAclId:         DefaultWAFWebAclId,
		nlbCrossZone:        DefaultNLBCrossZone,
		nlbHTTPEnabled:      DefaultNLBHTTPEnabled,
		customFilter:        DefaultCustomFilter,
	}

	adapter.manifest, err = buildManifest(adapter)
	if err != nil {
		return nil, err
	}

	return
}

func (a *Adapter) NewACMCertificateProvider() certs.CertificatesProvider {
	return newACMCertProvider(a.acm)
}

func (a *Adapter) NewIAMCertificateProvider() certs.CertificatesProvider {
	return newIAMCertProvider(a.iam)
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

// WithTargetPort returns the receiver adapter after changing the target port that will be used by
// the resources created by the adapter
func (a *Adapter) WithTargetPort(port uint) *Adapter {
	a.targetPort = port
	return a
}

// WithHealthCheckInterval returns the receiver adapter after changing the health check interval that will be used by
// the resources created by the adapter
func (a *Adapter) WithHealthCheckInterval(interval time.Duration) *Adapter {
	a.healthCheckInterval = interval
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

// WithWAFWebAclId returns the receiver adapter after changing the waf web acl id for waf association
func (a *Adapter) WithWAFWebAclId(wafWebAclId string) *Adapter {
	a.wafWebAclId = wafWebAclId
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

// ClusterID returns the ClusterID tag that all resources from the same Kubernetes cluster share.
// It's taken from the current ec2 instance.
func (a *Adapter) ClusterID() string {
	return a.manifest.instance.clusterID()
}

// VpcID returns the VPC ID the current node belongs to.
func (a *Adapter) VpcID() string {
	return a.manifest.instance.vpcID
}

// InstanceID returns the instance ID the current node is running on.
func (a *Adapter) InstanceID() string {
	return a.manifest.instance.id
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
		result += fmt.Sprintf("%s=%s ", aws.StringValue(filter.Name), strings.Join(aws.StringValueSlice(filter.Values), ","))
	}
	return strings.TrimSpace(result)
}

// SecurityGroupID returns the security group ID that should be used to create Load Balancers.
func (a *Adapter) SecurityGroupID() string {
	return a.manifest.securityGroup.id
}

// FindManagedStacks returns all CloudFormation stacks containing the controller management tags
// that match the current cluster and are ready to be used. The stack status is used to filter.
func (a *Adapter) FindManagedStacks() ([]*Stack, error) {
	stacks, err := findManagedStacks(a.cloudformation, a.ClusterID(), a.controllerID)
	if err != nil {
		return nil, err
	}
	return stacks, nil
}

// UpdateTargetGroupsAndAutoScalingGroups updates Auto Scaling Groups
// config to have relevant Target Groups and registers/deregisters single
// instances (that do not belong to ASG) in relevant Target Groups.
func (a *Adapter) UpdateTargetGroupsAndAutoScalingGroups(stacks []*Stack) {
	targetGroupARNs := make([]string, 0, len(stacks))
	for _, stack := range stacks {
		if stack.TargetGroupARN != "" {
			targetGroupARNs = append(targetGroupARNs, stack.TargetGroupARN)
		}
	}

	// don't do anything if there are no target groups
	if len(targetGroupARNs) == 0 {
		return
	}

	ownerTags := map[string]string{
		clusterIDTagPrefix + a.ClusterID(): resourceLifecycleOwned,
		kubernetesCreatorTag:               a.controllerID,
	}

	for _, asg := range a.TargetedAutoScalingGroups {
		// This call is idempotent and safe to execute every time
		if err := updateTargetGroupsForAutoScalingGroup(a.autoscaling, a.elbv2, targetGroupARNs, asg.name, ownerTags); err != nil {
			log.Errorf("UpdateTargetGroupsAndAutoScalingGroups() failed to attach target groups to ASG '%s': %v", asg.name, err)
		}
	}

	// remove owned TGs from non-targeted ASGs
	nonTargetedASGs := nonTargetedASGs(a.OwnedAutoScalingGroups, a.TargetedAutoScalingGroups)
	for _, asg := range nonTargetedASGs {
		// This call is idempotent and safe to execute every time
		if err := updateTargetGroupsForAutoScalingGroup(a.autoscaling, a.elbv2, nil, asg.name, ownerTags); err != nil {
			log.Errorf("UpdateTargetGroupsAndAutoScalingGroups() failed to attach target groups to ASG '%s': %v", asg.name, err)
		}
	}

	runningSingleInstances := a.RunningSingleInstances()
	if len(runningSingleInstances) != 0 {
		// This call is idempotent too
		if err := registerTargetsOnTargetGroups(a.elbv2, targetGroupARNs, runningSingleInstances); err != nil {
			log.Errorf("UpdateTargetGroupsAndAutoScalingGroups() failed to register instances %q in target groups: %v", runningSingleInstances, err)
		}
	}
	if len(a.obsoleteInstances) != 0 {
		// Deregister instances from target groups and clean up list of obsolete instances
		if err := deregisterTargetsOnTargetGroups(a.elbv2, targetGroupARNs, a.obsoleteInstances); err != nil {
			log.Errorf("UpdateTargetGroupsAndAutoScalingGroups() failed to deregister instances %q in target groups: %v", a.obsoleteInstances, err)
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
func (a *Adapter) CreateStack(certificateARNs []string, scheme, securityGroup, owner, sslPolicy, ipAddressType, wafWebACLId string, cwAlarms CloudWatchAlarmList, loadBalancerType string, http2 bool) (string, error) {
	certARNs := make(map[string]time.Time, len(certificateARNs))
	for _, arn := range certificateARNs {
		certARNs[arn] = time.Time{}
	}

	if sslPolicy == "" {
		sslPolicy = a.sslPolicy
	}

	if wafWebACLId == "" {
		wafWebACLId = a.wafWebAclId
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
		},
		targetPort:                   a.targetPort,
		timeoutInMinutes:             uint(a.creationTimeout.Minutes()),
		stackTerminationProtection:   a.stackTerminationProtection,
		idleConnectionTimeoutSeconds: uint(a.idleConnectionTimeout.Seconds()),
		controllerID:                 a.controllerID,
		sslPolicy:                    sslPolicy,
		ipAddressType:                ipAddressType,
		loadbalancerType:             loadBalancerType,
		albLogsS3Bucket:              a.albLogsS3Bucket,
		albLogsS3Prefix:              a.albLogsS3Prefix,
		wafWebAclId:                  wafWebACLId,
		cwAlarms:                     cwAlarms,
		httpRedirectToHTTPS:          a.httpRedirectToHTTPS,
		nlbCrossZone:                 a.nlbCrossZone,
		nlbHTTPEnabled:               a.nlbHTTPEnabled,
		http2:                        http2,
	}

	return createStack(a.cloudformation, spec)
}

func (a *Adapter) UpdateStack(stackName string, certificateARNs map[string]time.Time, scheme, sslPolicy, ipAddressType, wafWebACLId string, cwAlarms CloudWatchAlarmList, loadBalancerType string, http2 bool) (string, error) {
	if _, ok := SSLPolicies[sslPolicy]; !ok {
		return "", fmt.Errorf("invalid SSLPolicy '%s' defined", sslPolicy)
	}

	if wafWebACLId == "" {
		wafWebACLId = a.wafWebAclId
	}

	spec := &stackSpec{
		name:            stackName,
		scheme:          scheme,
		certificateARNs: certificateARNs,
		securityGroupID: a.SecurityGroupID(),
		subnets:         a.FindLBSubnets(scheme),
		vpcID:           a.VpcID(),
		clusterID:       a.ClusterID(),
		healthCheck: &healthCheck{
			path:     a.healthCheckPath,
			port:     a.healthCheckPort,
			interval: a.healthCheckInterval,
		},
		targetPort:                   a.targetPort,
		timeoutInMinutes:             uint(a.creationTimeout.Minutes()),
		stackTerminationProtection:   a.stackTerminationProtection,
		idleConnectionTimeoutSeconds: uint(a.idleConnectionTimeout.Seconds()),
		controllerID:                 a.controllerID,
		sslPolicy:                    sslPolicy,
		ipAddressType:                ipAddressType,
		loadbalancerType:             loadBalancerType,
		albLogsS3Bucket:              a.albLogsS3Bucket,
		albLogsS3Prefix:              a.albLogsS3Prefix,
		wafWebAclId:                  wafWebACLId,
		cwAlarms:                     cwAlarms,
		httpRedirectToHTTPS:          a.httpRedirectToHTTPS,
		nlbCrossZone:                 a.nlbCrossZone,
		nlbHTTPEnabled:               a.nlbHTTPEnabled,
		http2:                        http2,
	}

	return updateStack(a.cloudformation, spec)
}

func (a *Adapter) stackName() string {
	return normalizeStackName(a.ClusterID())
}

// GetStack returns the CloudFormation stack details with the name or ID from the argument
func (a *Adapter) GetStack(stackID string) (*Stack, error) {
	return getStack(a.cloudformation, stackID)
}

// DeleteStack deletes the CloudFormation stack with the given name
func (a *Adapter) DeleteStack(stack *Stack) error {
	for _, asg := range a.TargetedAutoScalingGroups {
		if err := detachTargetGroupsFromAutoScalingGroup(a.autoscaling, []string{stack.TargetGroupARN}, asg.name); err != nil {
			return fmt.Errorf("DeleteStack failed to detach: %v", err)
		}
	}

	return deleteStack(a.cloudformation, stack.Name)
}

func buildManifest(awsAdapter *Adapter) (*manifest, error) {
	var err error

	myID, err := awsAdapter.ec2metadata.GetMetadata("instance-id")
	if err != nil {
		return nil, err
	}
	instanceDetails, err := getInstanceDetails(awsAdapter.ec2, myID)
	if err != nil {
		return nil, err
	}

	clusterID := instanceDetails.clusterID()

	securityGroupDetails, err := findSecurityGroupWithClusterID(awsAdapter.ec2, clusterID, awsAdapter.controllerID)
	if err != nil {
		return nil, err
	}

	subnets, err := getSubnets(awsAdapter.ec2, instanceDetails.vpcID, clusterID)
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
	}, nil
}

// FindLBSubnets finds subnets for an ALB based on the scheme.
//
// It follows the same logic for finding subnets as the kube-controller-manager
// when finding subnets for ELBs used for services of type LoadBalancer.
// https://github.com/kubernetes/kubernetes/blob/65efeee64f772e0f38037e91a677138a335a7570/pkg/cloudprovider/providers/aws/aws.go#L2949-L3027
func (a *Adapter) FindLBSubnets(scheme string) []string {
	var internal bool
	if scheme == elbv2.LoadBalancerSchemeEnumInternal {
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
func (a *Adapter) UpdateAutoScalingGroupsAndInstances() error {
	var err error
	a.ec2Details, err = getInstancesDetailsWithFilters(a.ec2, a.manifest.filters)
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

	targetedASGs, ownedASGs, err := getOwnedAndTargetedAutoScalingGroups(a.autoscaling, a.manifest.asgFilters, ownedTag)
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
func (a *Adapter) parseFilters(clusterId string) []*ec2.Filter {
	if a.customFilter != "" {
		terms := strings.Fields(a.customFilter)
		filters := make([]*ec2.Filter, len(terms))
		for i, term := range terms {
			parts := strings.Split(term, "=")
			if len(parts) != 2 {
				log.Errorf("failed parsing %s, falling back to default", a.customFilter)
				return generateDefaultFilters(clusterId)
			}
			filters[i] = &ec2.Filter{
				Name:   aws.String(parts[0]),
				Values: aws.StringSlice(strings.Split(parts[1], ",")),
			}
		}
		return filters
	}
	return generateDefaultFilters(clusterId)
}

// Generate default EC2 filter for usage with ECs DescribeInstances call based on EC2 tags
// of instance where Ingress Controller pod was started.
func generateDefaultFilters(clusterId string) []*ec2.Filter {
	return []*ec2.Filter{
		{
			Name:   aws.String("tag:" + clusterIDTagPrefix + clusterId),
			Values: []*string{aws.String(resourceLifecycleOwned)},
		},
		{
			Name:   aws.String("tag-key"),
			Values: []*string{aws.String(kubernetesNodeRoleTag)},
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
				log.Errorf("failed parsing %s, falling back to default", a.customFilter)
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
