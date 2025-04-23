package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	log "github.com/sirupsen/logrus"
)

const (
	defaultClusterID           = "unknown-cluster"
	kubernetesClusterLegacyTag = "KubernetesCluster"
	clusterIDTag               = "ClusterID" // TODO(sszuecs): deprecated fallback cleanup
	clusterIDTagPrefix         = "kubernetes.io/cluster/"
	resourceLifecycleOwned     = "owned"
	kubernetesCreatorTag       = "kubernetes:application"
	autoScalingGroupNameTag    = "aws:autoscaling:groupName"
	runningState               = 16 // See https://github.com/aws/aws-sdk-go-v2/blob/main/service/ec2/types/types.go, type InstanceState
	stoppedState               = 80 // See https://github.com/aws/aws-sdk-go-v2/blob/main/service/ec2/types/types.go, type InstanceState
	elbRoleTagName             = "kubernetes.io/role/elb"
	internalELBRoleTagName     = "kubernetes.io/role/internal-elb"
	kubernetesNodeRoleTag      = "k8s.io/role/node"
)

type EC2IFaceAPI interface {
	DescribeInstances(context.Context, *ec2.DescribeInstancesInput, ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
	DescribeSubnets(context.Context, *ec2.DescribeSubnetsInput, ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error)
	DescribeRouteTables(context.Context, *ec2.DescribeRouteTablesInput, ...func(*ec2.Options)) (*ec2.DescribeRouteTablesOutput, error)
	DescribeSecurityGroups(context.Context, *ec2.DescribeSecurityGroupsInput, ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error)
}

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

func getInstanceDetails(ctx context.Context, ec2Service EC2IFaceAPI, instanceID string) (*instanceDetails, error) {
	params := &ec2.DescribeInstancesInput{
		Filters: []types.Filter{
			{
				Name: aws.String("instance-id"),
				Values: []string{
					instanceID,
				},
			},
		},
	}
	resp, err := ec2Service.DescribeInstances(ctx, params)
	if err != nil || resp == nil {
		return nil, fmt.Errorf("unable to get details for instance %q: %w", instanceID, err)
	}

	i, err := findFirstRunningInstance(resp)
	if err != nil {
		return nil, fmt.Errorf("unable to find instance %q: %w", instanceID, err)
	}

	return &instanceDetails{
		id:      aws.ToString(i.InstanceId),
		ip:      aws.ToString(i.PrivateIpAddress),
		vpcID:   aws.ToString(i.VpcId),
		tags:    convertEc2Tags(i.Tags),
		running: aws.ToInt32(i.State.Code)&0xff == runningState,
	}, nil
}

func getInstancesDetailsWithFilters(ctx context.Context, ec2Service EC2IFaceAPI, filters []types.Filter) (map[string]*instanceDetails, error) {
	params := &ec2.DescribeInstancesInput{
		Filters: filters,
	}
	result := make(map[string]*instanceDetails)
	paginator := ec2.NewDescribeInstancesPaginator(ec2Service, params)
	for paginator.HasMorePages() {
		resp, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed getting instance list from EC2: %w", err)
		}
		for _, reservation := range resp.Reservations {
			for _, instance := range reservation.Instances {
				result[aws.ToString(instance.InstanceId)] = &instanceDetails{
					id:      aws.ToString(instance.InstanceId),
					ip:      aws.ToString(instance.PrivateIpAddress),
					vpcID:   aws.ToString(instance.VpcId),
					tags:    convertEc2Tags(instance.Tags),
					running: aws.ToInt32(instance.State.Code)&0xff == runningState,
				}
			}
		}
	}
	return result, nil
}

func findFirstRunningInstance(resp *ec2.DescribeInstancesOutput) (*types.Instance, error) {
	for _, reservation := range resp.Reservations {
		for _, instance := range reservation.Instances {
			// The low byte represents the state. The high byte is an opaque internal value
			// and should be ignored.
			if aws.ToInt32(instance.State.Code)&0xff == runningState {
				return &instance, nil
			}
		}
	}
	return nil, ErrNoRunningInstances
}

func getSubnets(ctx context.Context, svc EC2IFaceAPI, vpcID, clusterID string) ([]*subnetDetails, error) {
	params := &ec2.DescribeSubnetsInput{
		Filters: []types.Filter{
			{
				Name: aws.String("vpc-id"),
				Values: []string{
					vpcID,
				},
			},
		},
	}
	resp, err := svc.DescribeSubnets(ctx, params)
	if err != nil {
		return nil, err
	}

	log.Debug("aws.getRouteTables")
	rt, err := getRouteTables(ctx, svc, vpcID)
	if err != nil {
		return nil, err
	}

	retAll := make([]*subnetDetails, len(resp.Subnets))
	retFiltered := make([]*subnetDetails, 0)
	for i, sn := range resp.Subnets {
		az := aws.ToString(sn.AvailabilityZone)
		subnetID := aws.ToString(sn.SubnetId)
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
		if _, ok := tags[clusterIDTagPrefix+clusterID]; ok {
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
		log.Warn("No tagged subnets found; considering all subnets. This is likely to be an error in future versions.")
		return retAll, nil
	}
	return retFiltered, nil
}

func convertEc2Tags(instanceTags []types.Tag) map[string]string {
	tags := make(map[string]string, len(instanceTags))
	for _, tagDescription := range instanceTags {
		tags[aws.ToString(tagDescription.Key)] = aws.ToString(tagDescription.Value)
	}
	return tags
}

func getRouteTables(ctx context.Context, svc EC2IFaceAPI, vpcID string) ([]types.RouteTable, error) {
	params := &ec2.DescribeRouteTablesInput{
		Filters: []types.Filter{
			{
				Name: aws.String("vpc-id"),
				Values: []string{
					vpcID,
				},
			},
		},
	}
	resp, err := svc.DescribeRouteTables(ctx, params)
	if err != nil {
		return nil, err
	}

	return resp.RouteTables, nil
}

// Copied from Kubernete's https://github.com/kubernetes/kubernetes/blob/master/pkg/cloudprovider/providers/aws/aws.go
func isSubnetPublic(rt []types.RouteTable, subnetID string) (bool, error) {
	var subnetTable *types.RouteTable
	for _, table := range rt {
		for _, assoc := range table.Associations {
			if aws.ToString(assoc.SubnetId) == subnetID {
				subnetTable = &table
				break
			}
		}
	}

	if subnetTable == nil {
		// If there is no explicit association, the subnet will be implicitly
		// associated with the VPC's main routing table.
		for _, table := range rt {
			for _, assoc := range table.Associations {
				if aws.ToBool(assoc.Main) {
					subnetTable = &table
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
		if strings.HasPrefix(aws.ToString(route.GatewayId), "igw") {
			return true, nil
		}
	}

	return false, nil
}

func findSecurityGroupWithClusterID(ctx context.Context, svc EC2IFaceAPI, clusterID string, controllerID string) (*securityGroupDetails, error) {
	params := &ec2.DescribeSecurityGroupsInput{
		Filters: []types.Filter{
			{
				Name: aws.String("tag:" + clusterIDTagPrefix + clusterID),
				Values: []string{
					resourceLifecycleOwned,
				},
			},
			{
				Name: aws.String("tag:" + kubernetesCreatorTag),
				Values: []string{
					controllerID,
				},
			},
		},
	}

	resp, err := svc.DescribeSecurityGroups(ctx, params)
	if err != nil {
		return nil, err
	}

	if len(resp.SecurityGroups) < 1 {
		return nil, fmt.Errorf("could not find security group that matches: %v", params.Filters)
	}

	sg := resp.SecurityGroups[0]
	return &securityGroupDetails{
		name: aws.ToString(sg.GroupName),
		id:   aws.ToString(sg.GroupId),
	}, nil
}
