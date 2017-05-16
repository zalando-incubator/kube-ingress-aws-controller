package aws

import (
	"errors"
	"time"

	"fmt"

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
	"github.com/zalando-incubator/kube-ingress-aws-controller/certs"
	"log"
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

	customTemplate      string
	manifest            *manifest
	healthCheckPath     string
	healthCheckPort     uint
	healthCheckInterval time.Duration
	creationTimeout     time.Duration
}

type manifest struct {
	securityGroup    *securityGroupDetails
	instance         *instanceDetails
	autoScalingGroup *autoScalingGroupDetails
	privateSubnets   []*subnetDetails
	publicSubnets    []*subnetDetails
}

type configProviderFunc func() client.ConfigProvider

const (
	DefaultHealthCheckPath           = "/kube-system/healthz"
	DefaultHealthCheckPort           = 9999
	DefaultHealthCheckInterval       = 10 * time.Second
	DefaultCertificateUpdateInterval = 30 * time.Minute
	DefaultCreationTimeout           = 5 * time.Minute

	clusterIDTag = "ClusterID"
	nameTag      = "Name"

	kubernetesCreatorTag   = "kubernetes:application"
	kubernetesCreatorValue = "kube-ingress-aws-controller"
	certificateARNTag      = "ingress:certificate-arn"
)

var (
	// ErrMissingSecurityGroup is used to signal that the required security group couldn't be found.
	ErrMissingSecurityGroup = errors.New("required security group was not found")
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
	// ErrFailedToParsePEM is used to signal that the PEM block for a certificate failed to be parsed
	ErrFailedToParsePEM = errors.New("failed to parse certificate PEM")
)

var configProvider = defaultConfigProvider

func defaultConfigProvider() client.ConfigProvider {
	cfg := aws.NewConfig().WithMaxRetries(3)
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
func NewAdapter() (adapter *Adapter, err error) {
	p := configProvider()
	adapter = &Adapter{
		elbv2:               elbv2.New(p),
		ec2:                 ec2.New(p),
		ec2metadata:         ec2metadata.New(p),
		autoscaling:         autoscaling.New(p),
		acm:                 acm.New(p),
		iam:                 iam.New(p),
		cloudformation:      cloudformation.New(p),
		healthCheckPath:     DefaultHealthCheckPath,
		healthCheckPort:     DefaultHealthCheckPort,
		healthCheckInterval: DefaultHealthCheckInterval,
		creationTimeout:     DefaultCreationTimeout,
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

// WithCustomTemplate returns the receiver adapter after changing the CloudFormation template that should be used
// to create Load Balancer stacks
func (a *Adapter) WithCustomTemplate(template string) *Adapter {
	a.customTemplate = template
	return a
}

// ClusterStackName returns the ClusterID tag that all resources from the same Kubernetes cluster share.
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

// AutoScalingGroupName returns the name of the Auto Scaling Group the current node belongs to
func (a *Adapter) AutoScalingGroupName() string {
	return a.manifest.autoScalingGroup.name
}

// SecurityGroupID returns the security group ID that should be used to create Load Balancers.
func (a *Adapter) SecurityGroupID() string {
	return a.manifest.securityGroup.id
}

// PrivateSubnetIDs returns a slice with the private subnet IDs discovered by the adapter.
func (a *Adapter) PrivateSubnetIDs() []string {
	return getSubnetIDs(a.manifest.privateSubnets)
}

// PublicSubnetIDs returns a slice with the public subnet IDs discovered by the adapter.
func (a *Adapter) PublicSubnetIDs() []string {
	return getSubnetIDs(a.manifest.publicSubnets)
}

// FindManagedStacks returns all CloudFormation stacks containing the controller management tags
// that match the current cluster and are ready to be used. The stack status is used to filter.
func (a *Adapter) FindManagedStacks() ([]*Stack, error) {
	stacks, err := findManagedStacks(a.cloudformation, a.ClusterID())
	if err != nil {
		return nil, err
	}
	targetGroupARNs := make([]string, len(stacks))
	for i, stack := range stacks {
		targetGroupARNs[i] = stack.targetGroupARN
	}
	// This call is idempotent and safe to execute every time
	if err := attachTargetGroupsToAutoScalingGroup(a.autoscaling, targetGroupARNs, a.AutoScalingGroupName()); err != nil {
		log.Printf("FindManagedStacks() failed to attach target groups to ASG: %v", err)
	}
	return stacks, nil
}

// CreateStack creates a new Application Load Balancer using CloudFormation. The stack name is derived
// from the Cluster ID and the certificate ARN (when available).
// All the required resources (listeners and target group) are created in a transactional fashion.
// Failure to create the stack causes it to be deleted automatically.
func (a *Adapter) CreateStack(certificateARN string) (string, error) {
	spec := &createStackSpec{
		name:            normalizeStackName(a.ClusterID(), certificateARN),
		scheme:          elbv2.LoadBalancerSchemeEnumInternetFacing,
		certificateARN:  certificateARN,
		securityGroupID: a.SecurityGroupID(),
		subnets:         a.PublicSubnetIDs(),
		vpcID:           a.VpcID(),
		clusterID:       a.ClusterID(),
		healthCheck: &healthCheck{
			path:     a.healthCheckPath,
			port:     a.healthCheckPort,
			interval: a.healthCheckInterval,
		},
		timeoutInMinutes: uint(a.creationTimeout.Minutes()),
	}

	return createStack(a.cloudformation, spec)
}

// GetStack returns the CloudFormation stack details with the name or ID from the argument
func (a *Adapter) GetStack(stackID string) (*Stack, error) {
	return getStack(a.cloudformation, stackID)
}

// DeleteStack deletes the CloudFormation stack with the given name
func (a *Adapter) DeleteStack(stack *Stack) error {
	if err := detachTargetGroupFromAutoScalingGroup(a.autoscaling, stack.TargetGroupARN(), a.AutoScalingGroupName()); err != nil {
		return fmt.Errorf("DeleteStack failed to dettach: %v", err)
	}

	return deleteStack(a.cloudformation, stack.Name())
}

func getSubnetIDs(snds []*subnetDetails) []string {
	ret := make([]string, len(snds))
	for i, snd := range snds {
		ret[i] = snd.id
	}
	return ret
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

	autoScalingGroupName, err := getAutoScalingGroupName(instanceDetails.tags)
	if err != nil {
		return nil, err
	}
	autoScalingGroupDetails, err := getAutoScalingGroupByName(awsAdapter.autoscaling, autoScalingGroupName)
	if err != nil {
		return nil, err
	}

	stackName := instanceDetails.name()

	securityGroupDetails, err := findSecurityGroupWithNameTag(awsAdapter.ec2, stackName)
	if err != nil {
		return nil, err
	}

	subnets, err := getSubnets(awsAdapter.ec2, instanceDetails.vpcID)
	if err != nil {
		return nil, err
	}
	if len(subnets) == 0 {
		return nil, ErrNoSubnets
	}

	var (
		priv []*subnetDetails
		pub  []*subnetDetails
	)

	for _, subnet := range subnets {
		if subnet.public {
			pub = append(pub, subnet)
		} else {
			priv = append(priv, subnet)
		}
	}

	return &manifest{
		securityGroup:    securityGroupDetails,
		instance:         instanceDetails,
		autoScalingGroup: autoScalingGroupDetails,
		privateSubnets:   priv,
		publicSubnets:    pub,
	}, nil
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
