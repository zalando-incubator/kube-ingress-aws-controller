package aws

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"strings"
)

type instanceDetails struct {
	id    string
	vpcID string
	tags  map[string]string
}

func (id *instanceDetails) name() string {
	if n, err := getNameTag(id.tags); err == nil {
		return n
	}
	return "unknown instance"

}

type subnetDetails struct {
	id               string
	availabilityZone string
	tags             map[string]string
	public           bool
}

type securityGroupDetails struct {
	name string
	id   string
}

func (sd *subnetDetails) String() string {
	return fmt.Sprintf("%s (%s) @ %s", sd.Name(), sd.id, sd.availabilityZone)
}

func (sd *subnetDetails) Name() string {
	if n, err := getNameTag(sd.tags); err == nil {
		return n
	}
	return "unknown subnet"
}

const (
	autoScalingGroupNameTag = "aws:autoscaling:groupName"
	runningState            = 16 // See github.com/aws/aws-sdk-go/service/ec2/api.go, type InstanceState
)

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
	if err != nil {
		return nil, err
	}

	if len(resp.Reservations) < 1 || len(resp.Reservations[0].Instances) < 1 {
		return nil, fmt.Errorf("unable to get details for instance %q", instanceID)
	}

	i := resp.Reservations[0].Instances[0]
	if aws.Int64Value(i.State.Code) != runningState {
		return nil, fmt.Errorf("instance is in an invalid state: %s", aws.StringValue(i.State.Name))
	}

	return &instanceDetails{
		id:    aws.StringValue(i.InstanceId),
		vpcID: aws.StringValue(i.VpcId),
		tags:  convertEc2Tags(i.Tags),
	}, nil
}

func getSubnets(svc ec2iface.EC2API, vpcID string) ([]*subnetDetails, error) {
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

	ret := make([]*subnetDetails, len(resp.Subnets))
	for i, sn := range resp.Subnets {
		az := aws.StringValue(sn.AvailabilityZone)
		subnetID := aws.StringValue(sn.SubnetId)
		isPublic, err := isSubnetPublic(rt, subnetID)
		if err != nil {
			return nil, err
		}
		ret[i] = &subnetDetails{
			id:               subnetID,
			availabilityZone: az,
			public:           isPublic,
			tags:             convertEc2Tags(sn.Tags),
		}
	}
	return ret, nil

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
				if aws.BoolValue(assoc.Main) == true {
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

func convertEc2Tags(instanceTags []*ec2.Tag) map[string]string {
	tags := make(map[string]string, len(instanceTags))
	for _, tagDescription := range instanceTags {
		tags[aws.StringValue(tagDescription.Key)] = aws.StringValue(tagDescription.Value)
	}
	return tags
}

func findSecurityGroupWithNameTag(svc ec2iface.EC2API, nameTag string) (*securityGroupDetails, error) {
	params := &ec2.DescribeSecurityGroupsInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("tag-key"),
				Values: []*string{
					aws.String("Name"),
				},
			},
			{
				Name: aws.String("tag-value"),
				Values: []*string{
					aws.String(nameTag),
				},
			},
			{
				Name: aws.String("tag-key"),
				Values: []*string{
					aws.String("aws:cloudformation:logical-id"),
				},
			},
			{
				Name: aws.String("tag-value"),
				Values: []*string{
					aws.String("IngressLoadBalancerSecurityGroup"),
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
