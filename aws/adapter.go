package aws

import (
	"errors"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/acm"
	"github.com/aws/aws-sdk-go/service/acm/acmiface"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/aws/aws-sdk-go/service/elbv2/elbv2iface"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/linki/instrumented_http"
)

// An Adapter can be used to orchestrate and obtain information from Amazon Web Services.
type Adapter struct {
	ec2metadata        *ec2metadata.EC2Metadata
	ec2                ec2iface.EC2API
	elbv2              elbv2iface.ELBV2API
	autoscaling        autoscalingiface.AutoScalingAPI
	acm                acmiface.ACMAPI
	iam                iamiface.IAMAPI
	manifest           *manifest
	healthCheckPath    string
	healthCheckPort    uint16
	certUpdateInterval time.Duration
}

type manifest struct {
	securityGroup    *securityGroupDetails
	instance         *instanceDetails
	autoScalingGroup *autoScalingGroupDetails
	privateSubnets   []*subnetDetails
	publicSubnets    []*subnetDetails
	certificateCache *certificateCache
}

type configProviderFunc func() client.ConfigProvider

const (
	clusterIDTag = "ClusterID"
	nameTag      = "Name"
)

var (
	// ErrMissingSecurityGroup is used to signal that the required security group couldn't be found.
	ErrMissingSecurityGroup = errors.New("required security group was not found")
	// ErrLoadBalancerNotFound is used to signal that a given load balancer was not found.
	ErrLoadBalancerNotFound = errors.New("load balancer not found")
	// ErrMissingNameTag is used to signal that the Name tag on a given resource is missing.
	ErrMissingNameTag = errors.New("Name tag not found")
	// ErrMissingTag is used to signal that a tag on a given resource is missing.
	ErrMissingTag = errors.New("missing tag")
	// ErrNoSubnets is used to signal that no subnets were found in the current VPC
	ErrNoSubnets = errors.New("unable to find VPC subnets")
	// ErrMissingAutoScalingGroupTag is used to signal that the auto scaling group tag is not present in the list of tags.
	ErrMissingAutoScalingGroupTag = errors.New(`instance is missing the "` + autoScalingGroupNameTag + `" tag`)
	// ErrNoMatchingCertificateFound is used if there is no matching ACM certificate found
	ErrNoMatchingCertificateFound = errors.New("no matching ACM certificate found")
	// ErrNoRunningInstances is used to signal that no instances were found in the running state
	ErrNoRunningInstances = errors.New("no reservations or instances in the running state matched the DescribeInstances request")
)

var configProvider = defaultConfigProvider

func defaultConfigProvider() client.ConfigProvider {
	config := aws.NewConfig().WithMaxRetries(3)
	config = config.WithHTTPClient(instrumented_http.NewClient(config.HTTPClient, nil))
	return session.Must(session.NewSession(config))
}

// NewAdapter returns a new Adapter that can be used to orchestrate and obtain information from Amazon Web Services.
// Before returning there is a discovery process for VPC and EC2 details. It tries to find the TargetGroup and
// Security Group that should be used for newly created LoadBalancers. If any of those critical steps fail
// an appropriate error is returned.
func NewAdapter(healthCheckPath string, healthCheckPort uint16, certUpdateInterval time.Duration) (adapter *Adapter, err error) {
	p := configProvider()
	adapter = &Adapter{
		elbv2:              elbv2.New(p),
		ec2:                ec2.New(p),
		ec2metadata:        ec2metadata.New(p),
		autoscaling:        autoscaling.New(p),
		acm:                acm.New(p),
		iam:                iam.New(p),
		healthCheckPath:    healthCheckPath,
		healthCheckPort:    healthCheckPort,
		certUpdateInterval: certUpdateInterval,
	}

	adapter.manifest, err = buildManifest(adapter)
	if err != nil {
		return nil, err
	}

	return
}

// GetCerts returns the list of certificates. It's taken from a
// cache. Right now only ACM certifcates are supported.
func (a *Adapter) GetCerts() []*CertDetail {
	return a.manifest.certificateCache.GetCachedCerts()
}

// FindBestMatchingCertificate returns the best matching certificate
// dependent on string match (required), NotBefore and NotAfter
// attributes of certificates. If there are more than one equally
// matching certifactes are found, then the best is most of the time
// the newest certificate, such that you can update and revoke your
// certificates.
func (a *Adapter) FindBestMatchingCertificate(certs []*CertDetail, hostname string) (*CertDetail, error) {
	return FindBestMatchingCertificate(certs, hostname)
}

// StackName returns the Name tag that all resources created by the same CloudFormation stack share. It's taken from
// The current ec2 instance.
func (a *Adapter) StackName() string {
	return a.manifest.instance.name()
}

// StackName returns the ClusterID tag that all resources from the same Kubernetes cluster share. It's taken from
// The current ec2 instance.
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

// FindLoadBalancerWithCertificateID looks up for the first Application Load Balancer with, at least, 1 listener with
// the certificateARN. Order is not guaranteed and depends only on the AWS SDK result order.
func (a *Adapter) FindLoadBalancerWithCertificateID(certificateARN string) (*LoadBalancer, error) {
	return findManagedLBWithCertificateID(a.elbv2, a.ClusterID(), certificateARN)
}

// FindManagedLoadBalancers returns all ALBs containing the controller management tags for the current cluster.
func (a *Adapter) FindManagedLoadBalancers() ([]*LoadBalancer, error) {
	lbs, err := findManagedLoadBalancers(a.elbv2, a.ClusterID())
	if err != nil {
		return nil, err
	}
	return lbs, nil
}

// CreateLoadBalancer creates a new Application Load Balancer with an HTTPS listener using the certificate with the
// certificateARN argument. It will forward all requests to the target group discovered by the Adapter.
func (a *Adapter) CreateLoadBalancer(certificateARN string) (*LoadBalancer, error) {
	// only internet-facing for now. Maybe we need a separate SG for internal vs internet-facing
	spec := &createLoadBalancerSpec{
		scheme:          elbv2.LoadBalancerSchemeEnumInternetFacing,
		certificateARN:  certificateARN,
		securityGroupID: a.SecurityGroupID(),
		stackName:       a.StackName(),
		subnets:         a.PublicSubnetIDs(),
		vpcID:           a.VpcID(),
		clusterID:       a.ClusterID(),
		healthCheck: healthCheck{
			path: a.healthCheckPath,
			port: a.healthCheckPort,
		},
	}
	var (
		lb  *LoadBalancer
		err error
	)

	lb, err = createLoadBalancer(a.elbv2, spec)
	if err != nil {
		return nil, err
	}

	if err := attachTargetGroupToAutoScalingGroup(a.autoscaling, lb.listeners.targetGroupARN, a.AutoScalingGroupName()); err != nil {
		// TODO: delete previously created load balancer?
		return nil, err
	}
	return lb, nil
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

	cc := newCertCache(newIAMCertProvider(awsAdapter.iam), newACMCertProvider(awsAdapter.acm))
	if err := cc.updateCertCache(); err != nil {
		return nil, err
	}
	cc.backgroundCertCacheUpdate(awsAdapter.certUpdateInterval)

	return &manifest{
		securityGroup:    securityGroupDetails,
		instance:         instanceDetails,
		autoScalingGroup: autoScalingGroupDetails,
		privateSubnets:   priv,
		publicSubnets:    pub,
		certificateCache: cc,
	}, nil
}

func (a *Adapter) DeleteLoadBalancer(loadBalancer *LoadBalancer) error {
	targetGroupARN := loadBalancer.listeners.targetGroupARN
	if loadBalancer.listeners.https != nil {
		if err := deleteListener(a.elbv2, loadBalancer.listeners.https.arn); err != nil {
			return err
		}
	}

	if loadBalancer.listeners.http != nil {
		if err := deleteListener(a.elbv2, loadBalancer.listeners.http.arn); err != nil {
			return err
		}
	}

	if err := detachTargetGroupFromAutoScalingGroup(a.autoscaling, targetGroupARN, a.manifest.autoScalingGroup.name); err != nil {
		return err
	}

	if err := deleteTargetGroup(a.elbv2, targetGroupARN); err != nil {
		return err
	}

	if err := deleteLoadBalancer(a.elbv2, loadBalancer.arn); err != nil {
		return err
	}

	return nil
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
