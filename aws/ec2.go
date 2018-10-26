package aws

import (
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
)

const (
	defaultClusterID           = "unknown-cluster"
	kubernetesClusterLegacyTag = "KubernetesCluster"
	clusterIDTag               = "ClusterID" // TODO(sszuecs): deprecated fallback cleanup
	clusterIDTagPrefix         = "kubernetes.io/cluster/"
	resourceLifecycleOwned     = "owned"
	kubernetesCreatorTag       = "kubernetes:application"
	autoScalingGroupNameTag    = "aws:autoscaling:groupName"
	runningState               = 16 // See https://github.com/aws/aws-sdk-go/blob/master/service/ec2/api.go, type InstanceState
	stoppedState               = 80 // See https://github.com/aws/aws-sdk-go/blob/master/service/ec2/api.go, type InstanceState
	elbRoleTagName             = "kubernetes.io/role/elb"
	internalELBRoleTagName     = "kubernetes.io/role/internal-elb"
	kubernetesNodeRoleTag      = "k8s.io/role/node"
)

type securityGroupDetails struct {
	name string
	id   string
}

type instanceDetails struct {
	id      string
	ip      string
	vpcID   string
	tags    map[string]string
	running bool
}

func (id *instanceDetails) clusterID() string {
	for name, value := range id.tags {
		if strings.HasPrefix(name, clusterIDTagPrefix) && value == resourceLifecycleOwned {
			return strings.TrimPrefix(name, clusterIDTagPrefix)
		}
	}
	for name, value := range id.tags {
		if name == kubernetesClusterLegacyTag {
			return value
		}
	}
	return defaultClusterID
}

type subnetDetails struct {
	id               string
	availabilityZone string
	tags             map[string]string
	public           bool
}

func (sd *subnetDetails) String() string {
	return fmt.Sprintf("%s (%s) @ %s (public: %t)", sd.Name(), sd.id, sd.availabilityZone, sd.public)
}

func (sd *subnetDetails) Name() string {
	if n, err := getNameTag(sd.tags); err == nil {
		return n
	}
	return "unknown-subnet"
}

func getAutoScalingGroupName(instanceTags map[string]string) (string, error) {
	if len(instanceTags) < 1 {
		return "", ErrMissingAutoScalingGroupTag
	}

	asg, has := instanceTags[autoScalingGroupNameTag]
	if !has || asg == "" {
		return "", ErrMissingAutoScalingGroupTag
	}

	return asg, nil
}

func getInstanceDetails(ec2Service ec2iface.EC2API, instanceID string) (*instanceDetails, error) {
	params := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("instance-id"),
				Values: []*string{
					aws.String(instanceID),
				},
			},
		},
	}
	resp, err := ec2Service.DescribeInstances(params)
	if err != nil || resp == nil {
		return nil, fmt.Errorf("unable to get details for instance %q: %v", instanceID, err)
	}

	i, err := findFirstRunningInstance(resp)
	if err != nil {
		return nil, fmt.Errorf("unable to find instance %q: %v", instanceID, err)
	}

	return &instanceDetails{
		id:      aws.StringValue(i.InstanceId),
		ip:      aws.StringValue(i.PrivateIpAddress),
		vpcID:   aws.StringValue(i.VpcId),
		tags:    convertEc2Tags(i.Tags),
		running: aws.Int64Value(i.State.Code)&0xff == runningState,
	}, nil
}

func getInstancesDetailsWithFilters(ec2Service ec2iface.EC2API, filters []*ec2.Filter) (map[string]*instanceDetails, error) {
	params := &ec2.DescribeInstancesInput{
		Filters: filters,
	}
	result := make(map[string]*instanceDetails)
	err := ec2Service.DescribeInstancesPages(params, func(resp *ec2.DescribeInstancesOutput, lastPage bool) bool {
		for _, reservation := range resp.Reservations {
			for _, instance := range reservation.Instances {
				result[aws.StringValue(instance.InstanceId)] = &instanceDetails{
					id:      aws.StringValue(instance.InstanceId),
					ip:      aws.StringValue(instance.PrivateIpAddress),
					vpcID:   aws.StringValue(instance.VpcId),
					tags:    convertEc2Tags(instance.Tags),
					running: aws.Int64Value(instance.State.Code)&0xff == runningState,
				}
			}
		}
		return true
	})
	if err != nil {
		return nil, fmt.Errorf("failed getting instance list from EC2: %v", err)
	}

	return result, nil
}

func findFirstRunningInstance(resp *ec2.DescribeInstancesOutput) (*ec2.Instance, error) {
	for _, reservation := range resp.Reservations {
		for _, instance := range reservation.Instances {
			// The low byte represents the state. The high byte is an opaque internal value
			// and should be ignored.
			if aws.Int64Value(instance.State.Code)&0xff == runningState {
				return instance, nil
			}
		}
	}
	return nil, ErrNoRunningInstances
}

func getSubnets(svc ec2iface.EC2API, vpcID, clusterID string) ([]*subnetDetails, error) {
	params := &ec2.DescribeSubnetsInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("vpc-id"),
				Values: []*string{
					aws.String(vpcID),
				},
			},
		},
	}
	resp, err := svc.DescribeSubnets(params)
	if err != nil {
		return nil, err
	}

	rt, err := getRouteTables(svc, vpcID)
	if err != nil {
		return nil, err
	}

	retAll      := make([]*subnetDetails, len(resp.Subnets))
	retFiltered := make([]*subnetDetails, 0)
	for i, sn := range resp.Subnets {
		az := aws.StringValue(sn.AvailabilityZone)
		subnetID := aws.StringValue(sn.SubnetId)
		isPublic, err := isSubnetPublic(rt, subnetID)
		if err != nil {
			return nil, err
		}
		tags := convertEc2Tags(sn.Tags)
		retAll[i] = &subnetDetails{
			id:               subnetID,
			availabilityZone: az,
			public:           isPublic,
			tags:             tags,
		}
		if _, ok := tags[clusterIDTagPrefix + clusterID]; ok {
			retFiltered = append(retFiltered, &subnetDetails{
				id:               subnetID,
				availabilityZone: az,
				public:           isPublic,
				tags:             tags,
			})
		}
	}
	// Fall back to full list of subnets if none matching expected tagging are found, with a stern warning
	// https://github.com/kubernetes/kubernetes/blob/v1.10.3/pkg/cloudprovider/providers/aws/aws.go#L3009
	if len(retFiltered) == 0 {
		log.Printf("No tagged subnets found; considering all subnets. This is likely to be an error in future versions.")
		return retAll, nil
	}
	return retFiltered, nil
}

func convertEc2Tags(instanceTags []*ec2.Tag) map[string]string {
	tags := make(map[string]string, len(instanceTags))
	for _, tagDescription := range instanceTags {
		tags[aws.StringValue(tagDescription.Key)] = aws.StringValue(tagDescription.Value)
	}
	return tags
}

func getRouteTables(svc ec2iface.EC2API, vpcID string) ([]*ec2.RouteTable, error) {
	params := &ec2.DescribeRouteTablesInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("vpc-id"),
				Values: []*string{
					aws.String(vpcID),
				},
			},
		},
	}
	resp, err := svc.DescribeRouteTables(params)
	if err != nil {
		return nil, err
	}

	return resp.RouteTables, nil
}

// Copied from Kubernete's https://github.com/kubernetes/kubernetes/blob/master/pkg/cloudprovider/providers/aws/aws.go
func isSubnetPublic(rt []*ec2.RouteTable, subnetID string) (bool, error) {
	var subnetTable *ec2.RouteTable
	for _, table := range rt {
		for _, assoc := range table.Associations {
			if aws.StringValue(assoc.SubnetId) == subnetID {
				subnetTable = table
				break
			}
		}
	}

	if subnetTable == nil {
		// If there is no explicit association, the subnet will be implicitly
		// associated with the VPC's main routing table.
		for _, table := range rt {
			for _, assoc := range table.Associations {
				if aws.BoolValue(assoc.Main) {
					subnetTable = table
					break
				}
			}
		}
	}

	if subnetTable == nil {
		return false, fmt.Errorf("could not locate routing table for subnet %q", subnetID)
	}

	for _, route := range subnetTable.Routes {
		// There is no direct way in the AWS API to determine if a subnet is public or private.
		// A public subnet is one which has an internet gateway route
		// we look for the gatewayId and make sure it has the prefix of igw to differentiate
		// from the default in-subnet route which is called "local"
		// or other virtual gateway (starting with vgv)
		// or vpc peering connections (starting with pcx).
		if strings.HasPrefix(aws.StringValue(route.GatewayId), "igw") {
			return true, nil
		}
	}

	return false, nil
}

func findSecurityGroupWithClusterID(svc ec2iface.EC2API, clusterID string, controllerID string) (*securityGroupDetails, error) {
	params := &ec2.DescribeSecurityGroupsInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("tag-key"),
				Values: []*string{
					aws.String(clusterIDTagPrefix + clusterID),
				},
			},
			{
				Name: aws.String("tag-value"),
				Values: []*string{
					aws.String(resourceLifecycleOwned),
				},
			},
			{
				Name: aws.String("tag-key"),
				Values: []*string{
					aws.String(kubernetesCreatorTag),
				},
			},
			{
				Name: aws.String("tag-value"),
				Values: []*string{
					aws.String(controllerID),
				},
			},
		},
	}

	resp, err := svc.DescribeSecurityGroups(params)
	if err != nil {
		return nil, err
	}

	if len(resp.SecurityGroups) < 1 {
		return nil, ErrMissingSecurityGroup
	}

	sg := resp.SecurityGroups[0]
	return &securityGroupDetails{
		name: aws.StringValue(sg.GroupName),
		id:   aws.StringValue(sg.GroupId),
	}, nil
}
