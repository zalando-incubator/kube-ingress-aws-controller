package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
)

const dipSplitSize = 2

type ec2MockOutputs struct {
	describeSecurityGroups *apiResponse
	describeInstances      *apiResponse
	describeInstancesPages []*apiResponse
	describeSubnets        *apiResponse
	describeRouteTables    *apiResponse
}

type mockEc2Client struct {
	ec2iface.EC2API
	outputs ec2MockOutputs
}

func (m *mockEc2Client) DescribeSecurityGroups(*ec2.DescribeSecurityGroupsInput) (*ec2.DescribeSecurityGroupsOutput, error) {
	if out, ok := m.outputs.describeSecurityGroups.response.(*ec2.DescribeSecurityGroupsOutput); ok {
		return out, m.outputs.describeSecurityGroups.err
	}
	return nil, m.outputs.describeSecurityGroups.err
}

func (m *mockEc2Client) DescribeInstances(*ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error) {
	if out, ok := m.outputs.describeInstances.response.(*ec2.DescribeInstancesOutput); ok {
		return out, m.outputs.describeInstances.err
	}
	return nil, m.outputs.describeInstances.err
}

func (m *mockEc2Client) DescribeInstancesPages(params *ec2.DescribeInstancesInput, f func(*ec2.DescribeInstancesOutput, bool) bool) error {
	for _, resp := range m.outputs.describeInstancesPages {
		if out, ok := resp.response.(*ec2.DescribeInstancesOutput); ok {
			f(out, true)
		}
	}
	if len(m.outputs.describeInstancesPages) != 0 {
		return m.outputs.describeInstancesPages[0].err
	}
	return nil
}

func (m *mockEc2Client) DescribeSubnets(*ec2.DescribeSubnetsInput) (*ec2.DescribeSubnetsOutput, error) {
	if out, ok := m.outputs.describeSubnets.response.(*ec2.DescribeSubnetsOutput); ok {
		return out, m.outputs.describeSubnets.err
	}
	return nil, m.outputs.describeSubnets.err
}

func (m *mockEc2Client) DescribeRouteTables(*ec2.DescribeRouteTablesInput) (*ec2.DescribeRouteTablesOutput, error) {
	if out, ok := m.outputs.describeRouteTables.response.(*ec2.DescribeRouteTablesOutput); ok {
		return out, m.outputs.describeRouteTables.err
	}
	return nil, m.outputs.describeRouteTables.err
}

func mockDSGOutput(sgs map[string]string) *ec2.DescribeSecurityGroupsOutput {
	groups := make([]*ec2.SecurityGroup, 0)
	for id, name := range sgs {
		sg := &ec2.SecurityGroup{
			GroupId:   aws.String(id),
			GroupName: aws.String(name),
		}
		groups = append(groups, sg)
	}
	return &ec2.DescribeSecurityGroupsOutput{SecurityGroups: groups}
}

type testInstance struct {
	id        string
	tags      tags
	privateIp string
	vpcId     string
	state     int64
}

func mockDIOutput(mockedInstances ...testInstance) *ec2.DescribeInstancesOutput {
	instances := make([]*ec2.Instance, 0, len(mockedInstances))
	for _, i := range mockedInstances {
		tags := make([]*ec2.Tag, 0, len(i.tags))
		for k, v := range i.tags {
			tags = append(tags, &ec2.Tag{Key: aws.String(k), Value: aws.String(v)})
		}
		instance := &ec2.Instance{
			InstanceId:       aws.String(i.id),
			Tags:             tags,
			State:            &ec2.InstanceState{Code: aws.Int64(i.state)},
			PrivateIpAddress: aws.String(i.privateIp),
			VpcId:            aws.String(i.vpcId),
		}
		instances = append(instances, instance)
	}
	return &ec2.DescribeInstancesOutput{Reservations: []*ec2.Reservation{{Instances: instances}}}
}

func mockDIPOutput(e error, mockedInstances ...testInstance) []*apiResponse {
	pages := len(mockedInstances) / dipSplitSize
	result := make([]*apiResponse, pages, pages+1)
	for i := 0; i < pages; i++ {
		result[i] = R(mockDIOutput(mockedInstances[i*dipSplitSize:(i+1)*dipSplitSize]...), e)
	}
	if len(mockedInstances)%dipSplitSize != 0 {
		result = append(result, R(mockDIOutput(mockedInstances[pages*dipSplitSize:]...), e))
	}
	return result
}

type testSubnet struct {
	id   string
	az   string
	name string
	tags map[string]string
}

func mockDSOutput(mockedSubnets ...testSubnet) *ec2.DescribeSubnetsOutput {
	subnets := make([]*ec2.Subnet, 0, len(mockedSubnets))
	for _, subnet := range mockedSubnets {
		s := &ec2.Subnet{
			SubnetId:         aws.String(subnet.id),
			AvailabilityZone: aws.String(subnet.az),
			Tags: []*ec2.Tag{
				{Key: aws.String(nameTag), Value: aws.String(subnet.name)},
			},
		}
		for k, v := range subnet.tags {
			s.Tags = append(s.Tags, &ec2.Tag{Key: aws.String(k), Value: aws.String(v)})
		}
		subnets = append(subnets, s)
	}
	return &ec2.DescribeSubnetsOutput{Subnets: subnets}
}

type testRouteTable struct {
	subnetID   string
	main       bool
	gatewayIds []string
}

func mockDRTOutput(mockedRouteTables ...testRouteTable) *ec2.DescribeRouteTablesOutput {
	routeTables := make([]*ec2.RouteTable, 0, len(mockedRouteTables))
	for _, mrt := range mockedRouteTables {
		routes := make([]*ec2.Route, 0, len(mrt.gatewayIds))
		for _, gwID := range mrt.gatewayIds {
			routes = append(routes, &ec2.Route{GatewayId: aws.String(gwID)})
		}
		rt := &ec2.RouteTable{
			Associations: []*ec2.RouteTableAssociation{
				{SubnetId: aws.String(mrt.subnetID), Main: aws.Bool(mrt.main)},
			},
			Routes: routes,
		}
		routeTables = append(routeTables, rt)
	}
	return &ec2.DescribeRouteTablesOutput{RouteTables: routeTables}
}
