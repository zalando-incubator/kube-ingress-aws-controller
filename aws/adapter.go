package aws

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/aws/aws-sdk-go/service/elbv2/elbv2iface"
)

// An Adapter can be used to orchestrate and obtain information from Amazon Web Services
type Adapter struct {
	ec2metadata *ec2metadata.EC2Metadata
	ec2         ec2iface.EC2API
	elbv2       elbv2iface.ELBV2API
	autoscaling autoscalingiface.AutoScalingAPI
	manifest    *manifest
}

type manifest struct {
	securityGroup    *securityGroupDetails
	instance         *instanceDetails
	autoScalingGroup *autoScalingGroupDetails
	privateSubnets   []*subnetDetails
	publicSubnets    []*subnetDetails
}

// NewAdapter returns a new Adapter that can be used to orchestrate and obtain information from Amazon Web Services
// It accepts a manually set Auto Scaling Group name as an argument. If empty it will be discovered from the current node
// It also accepts the security group ID that should be used for newly created Load Balancers. If empty it will be derived
// from the Kubernetes cluster the current node belongs to. For ex.: kube1-worker-lb for a k8s cluster named kube1.
// Before returning there is a discovery process for VPC and EC2 details. If any of those critical steps fail
// an appropriate error is returned
func NewAdapter(p client.ConfigProvider, autoScalingGroupName string, securityGroupName string) (*Adapter, error) {
	elbv2 := elbv2.New(p)
	ec2 := ec2.New(p)
	ec2metadata := ec2metadata.New(p)
	autoscaling := autoscaling.New(p)

	adapter := &Adapter{
		elbv2:       elbv2,
		ec2:         ec2,
		ec2metadata: ec2metadata,
		autoscaling: autoscaling,
	}

	manifest, err := buildManifest(adapter, autoScalingGroupName, securityGroupName)
	if err != nil {
		return nil, err
	}
	adapter.manifest = manifest

	return adapter, nil
}

// VpcID returns the VPC ID the current node belongs to
func (a *Adapter) VpcID() string {
	return a.manifest.instance.vpcID
}

// InstanceID returns the instance ID the current node is running on
func (a *Adapter) InstanceID() string {
	return a.manifest.instance.id
}

// AutoScalingGroupName returns the name of the Auto Scaling Group the current node belongs to
func (a *Adapter) AutoScalingGroupName() string {
	return a.manifest.autoScalingGroup.name
}

// SecurityGroupID returns the security group ID that should be used to create Load Balancers
func (a *Adapter) SecurityGroupID() string {
	return a.manifest.securityGroup.id
}

// ClusterName returns the Kubernetes cluster name the current node belongs to
func (a *Adapter) ClusterName() string {
	return getClusterName(a.manifest.instance.tags)
}

// PrivateSubnetIDs returns a slice with the private subnet IDs discovered by the adapter
func (a *Adapter) PrivateSubnetIDs() []string {
	return getSubnetIDs(a.manifest.privateSubnets)
}

// PublicSubnetIDs returns a slice with the public subnet IDs discovered by the adapter
func (a *Adapter) PublicSubnetIDs() []string {
	return getSubnetIDs(a.manifest.publicSubnets)
}

// FindLoadBalancerWithCertificateID looks up for the first Application Load Balancer with, at least, 1 listener with
// the certificateARN. Order is not guaranteed and depends only on the AWS SDK result order
func (a *Adapter) FindLoadBalancerWithCertificateID(certificateARN string) (*LoadBalancer, error) {
	return findLoadBalancerWithCertificateID(a.elbv2, certificateARN)
}

// CreateLoadBalancer creates a new Application Load Balancer with an HTTPS listener using the certificate with the
// certificateARN argument. It will forward all requests to the target group discovered by the Adapter
func (a *Adapter) CreateLoadBalancer(certificateARN string) (*LoadBalancer, error) {
	// only internet-facing for now. Maybe we need a separate SG for internal vs internet-facing
	spec := &createLoadBalancerSpec{
		scheme:          elbv2.LoadBalancerSchemeEnumInternetFacing,
		certificateARN:  certificateARN,
		securityGroupID: a.SecurityGroupID(),
		clusterName:     a.ClusterName(),
		subnets:         a.PublicSubnetIDs(),
		targetGroupARNs: a.manifest.autoScalingGroup.targetGroups,
	}
	var (
		lb  *LoadBalancer
		err error
	)

	lb, err = createLoadBalancer(a.elbv2, spec)
	if err != nil {
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

func buildManifest(awsAdapter *Adapter, autoScalingGroupName string, securityGroupName string) (*manifest, error) {
	var err error

	myID, err := awsAdapter.ec2metadata.GetMetadata("instance-id")
	if err != nil {
		return nil, err
	}
	instanceDetails, err := getInstanceDetails(awsAdapter.ec2, myID)
	if err != nil {
		return nil, err
	}
	clusterName := getClusterName(instanceDetails.tags)

	if autoScalingGroupName == "" {
		autoScalingGroupName, err = getAutoScalingGroupName(instanceDetails.tags)
		if err != nil {
			return nil, err
		}
	}

	autoScalingGroupDetails, err := getAutoScalingGroupByName(awsAdapter.autoscaling, autoScalingGroupName)
	if err != nil {
		return nil, err
	}

	if len(autoScalingGroupDetails.targetGroups) < 1 {
		targetGroups, err := createDefaultTargetGroup(awsAdapter.elbv2, clusterName,
			instanceDetails.vpcID)
		if err != nil {
			return nil, err
		}
		if err := attachTargetGroupToAutoScalingGroup(awsAdapter.autoscaling, targetGroups, autoScalingGroupName); err != nil {
			return nil, err
		}
		autoScalingGroupDetails.targetGroups = targetGroups
	}

	if securityGroupName == "" {
		securityGroupName = fmt.Sprintf("%s-worker-lb", clusterName)
	}
	var securityGroupDetails *securityGroupDetails
	securityGroupDetails, err = getSecurityGroupByName(awsAdapter.ec2, securityGroupName)
	if err != nil {
		if err == ErrSecurityGroupNotFound {
			securityGroupDetails, err = createDefaultSecurityGroup(awsAdapter.ec2, securityGroupName, instanceDetails.vpcID)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	subnets, err := getSubnets(awsAdapter.ec2, instanceDetails.vpcID)
	if err != nil {
		return nil, err
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
		instance:         instanceDetails,
		autoScalingGroup: autoScalingGroupDetails,
		securityGroup:    securityGroupDetails,
		privateSubnets:   priv,
		publicSubnets:    pub,
	}, nil
}
