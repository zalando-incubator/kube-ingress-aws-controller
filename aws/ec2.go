package aws

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/pkg/errors"
	"strings"
)

type instanceDetails struct {
	id    string
	vpcID string
	tags  map[string]string
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
	for key, value := range sd.tags {
		if key == "Name" {
			return value
		}
	}
	return "<no name tag>"
}

const (
	autoScalingGroupNameTag = "aws:autoscaling:groupName"
	kubernetesClusterTag    = "KubernetesCluster"
)

var (
	// ErrSecurityGroupNotFound is used to signal that a given security group couldn't be found
	ErrSecurityGroupNotFound = errors.New("security group not found")
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

func getIntanceTags(ec2Service *ec2.EC2, instanceID string) (map[string]string, error) {
	params := &ec2.DescribeTagsInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("resource-id"),
				Values: []*string{
					aws.String(instanceID),
				},
			},
		},
	}
	resp, err := ec2Service.DescribeTags(params)
	if err != nil {
		return nil, err
	}
	tags := make(map[string]string, len(resp.Tags))
	for _, tagDescription := range resp.Tags {
		tags[aws.StringValue(tagDescription.Key)] = aws.StringValue(tagDescription.Value)
	}

	return tags, nil
}

func convertEc2Tags(instanceTags []*ec2.Tag) map[string]string {
	tags := make(map[string]string, len(instanceTags))
	for _, tagDescription := range instanceTags {
		tags[aws.StringValue(tagDescription.Key)] = aws.StringValue(tagDescription.Value)
	}
	return tags
}

func getClusterName(instanceTags map[string]string) string {
	if cn, has := instanceTags[kubernetesClusterTag]; has && len(cn) > 0 {
		return cn
	}

	return "" // TODO: fallback to some other k8s provided property?
}

func getSecurityGroupByName(svc ec2iface.EC2API, securityGroupName string) (*securityGroupDetails, error) {
	params := &ec2.DescribeSecurityGroupsInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("group-name"),
				Values: []*string{
					aws.String(securityGroupName),
				},
			},
		},
	}

	resp, err := svc.DescribeSecurityGroups(params)
	if err != nil {
		return nil, err
	}

	if len(resp.SecurityGroups) < 1 {
		return nil, ErrSecurityGroupNotFound
	}

	return &securityGroupDetails{
		name: securityGroupName,
		id:   aws.StringValue(resp.SecurityGroups[0].GroupId),
	}, nil
}

func createDefaultSecurityGroup(svc ec2iface.EC2API, securityGroupName string, vpcID string) (*securityGroupDetails, error) {
	params := &ec2.CreateSecurityGroupInput{
		GroupName:   aws.String(securityGroupName),
		VpcId:       aws.String(vpcID),
		Description: aws.String("Default security group for Kubernetes ALBs"),
	}
	resp, err := svc.CreateSecurityGroup(params)
	if err != nil {
		return nil, err
	}

	if err := addDefaultIngressRuleToSecurityGroup(svc, securityGroupName, resp.GroupId); err != nil {
		// TODO: delete newly created security group?
		return nil, err
	}

	return &securityGroupDetails{
		id:   aws.StringValue(resp.GroupId),
		name: securityGroupName,
	}, nil
}

func addDefaultIngressRuleToSecurityGroup(svc ec2iface.EC2API, securityGroupName string, securityGroupID *string) error {
	params := &ec2.AuthorizeSecurityGroupIngressInput{
		GroupId:    securityGroupID,
		GroupName:  aws.String(securityGroupName),
		IpProtocol: aws.String("tcp"),
		FromPort:   aws.Int64(443),
		ToPort:     aws.Int64(443),
		CidrIp:     aws.String("0.0.0.0/0"),
	}

	_, err := svc.AuthorizeSecurityGroupIngress(params)
	if err != nil {
		return err
	}

	return nil
}
