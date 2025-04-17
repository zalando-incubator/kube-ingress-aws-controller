package fake

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

const dipSplitSize = 2

type EC2Outputs struct {
	DescribeSecurityGroups *APIResponse
	DescribeInstances      *APIResponse
	DescribeInstancesPages []*APIResponse
	DescribeSubnets        *APIResponse
	DescribeRouteTables    *APIResponse
}

type EC2Client struct {
	Outputs EC2Outputs
}

func (m *EC2Client) DescribeSecurityGroups(context.Context, *ec2.DescribeSecurityGroupsInput, ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
	out, ok := m.Outputs.DescribeSecurityGroups.response.(*ec2.DescribeSecurityGroupsOutput)
	if !ok {
		return nil, m.Outputs.DescribeSecurityGroups.err
	}
	return out, m.Outputs.DescribeSecurityGroups.err
}

func (m *EC2Client) DescribeInstances(context.Context, *ec2.DescribeInstancesInput, ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	out, ok := m.Outputs.DescribeInstances.response.(*ec2.DescribeInstancesOutput)
	if !ok {
		return nil, m.Outputs.DescribeInstances.err
	}
	return out, m.Outputs.DescribeInstances.err
}

func (m *EC2Client) DescribeInstancesPages(ctx context.Context, params *ec2.DescribeInstancesInput, f func(*ec2.DescribeInstancesOutput, bool) bool) error {
	for _, resp := range m.Outputs.DescribeInstancesPages {
		if out, ok := resp.response.(*ec2.DescribeInstancesOutput); ok {
			f(out, true)
		}
	}
	if len(m.Outputs.DescribeInstancesPages) != 0 {
		return m.Outputs.DescribeInstancesPages[0].err
	}
	return nil
}

func (m *EC2Client) DescribeSubnets(context.Context, *ec2.DescribeSubnetsInput, ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
	out, ok := m.Outputs.DescribeSubnets.response.(*ec2.DescribeSubnetsOutput)
	if !ok {
		return nil, m.Outputs.DescribeSubnets.err
	}
	return out, m.Outputs.DescribeSubnets.err
}

func (m *EC2Client) DescribeRouteTables(context.Context, *ec2.DescribeRouteTablesInput, ...func(*ec2.Options)) (*ec2.DescribeRouteTablesOutput, error) {
	out, ok := m.Outputs.DescribeRouteTables.response.(*ec2.DescribeRouteTablesOutput)
	if !ok {
		return nil, m.Outputs.DescribeRouteTables.err
	}
	return out, m.Outputs.DescribeRouteTables.err
}

func MockDescribeSecurityGroupsOutput(sgs map[string]string) *ec2.DescribeSecurityGroupsOutput {
	groups := make([]types.SecurityGroup, 0)
	for id, name := range sgs {
		sg := types.SecurityGroup{
			GroupId:   aws.String(id),
			GroupName: aws.String(name),
		}
		groups = append(groups, sg)
	}
	return &ec2.DescribeSecurityGroupsOutput{SecurityGroups: groups}
}

type TestInstance struct {
	Id        string
	Tags      Tags
	PrivateIp string
	VpcId     string
	State     int64
}

func MockDescribeInstancesOutput(mockedInstances ...TestInstance) *ec2.DescribeInstancesOutput {
	instances := make([]types.Instance, 0, len(mockedInstances))
	for _, i := range mockedInstances {
		tags := make([]types.Tag, 0, len(i.Tags))
		for k, v := range i.Tags {
			tags = append(tags, types.Tag{Key: aws.String(k), Value: aws.String(v)})
		}
		instance := types.Instance{
			InstanceId:       aws.String(i.Id),
			Tags:             tags,
			State:            &types.InstanceState{Code: aws.Int32(int32(i.State))},
			PrivateIpAddress: aws.String(i.PrivateIp),
			VpcId:            aws.String(i.VpcId),
		}
		instances = append(instances, instance)
	}
	return &ec2.DescribeInstancesOutput{Reservations: []types.Reservation{{Instances: instances}}}
}

func MockDescribeInstancesPagesOutput(e error, mockedInstances ...TestInstance) []*APIResponse {
	pages := len(mockedInstances) / dipSplitSize
	result := make([]*APIResponse, pages, pages+1)
	for i := 0; i < pages; i++ {
		result[i] = R(MockDescribeInstancesOutput(mockedInstances[i*dipSplitSize:(i+1)*dipSplitSize]...), e)
	}
	if len(mockedInstances)%dipSplitSize != 0 {
		result = append(result, R(MockDescribeInstancesOutput(mockedInstances[pages*dipSplitSize:]...), e))
	}
	return result
}

type TestSubnet struct {
	Id   string
	Az   string
	Name string
	Tags map[string]string
}

func MockDescribeSubnetsOutput(mockedSubnets ...TestSubnet) *ec2.DescribeSubnetsOutput {
	subnets := make([]types.Subnet, 0, len(mockedSubnets))
	for _, subnet := range mockedSubnets {
		s := types.Subnet{
			SubnetId:         aws.String(subnet.Id),
			AvailabilityZone: aws.String(subnet.Az),
			Tags: []types.Tag{
				{Key: aws.String("Name"), Value: aws.String(subnet.Name)},
			},
		}
		for k, v := range subnet.Tags {
			s.Tags = append(s.Tags, types.Tag{Key: aws.String(k), Value: aws.String(v)})
		}
		subnets = append(subnets, s)
	}
	return &ec2.DescribeSubnetsOutput{Subnets: subnets}
}

type TestRouteTable struct {
	SubnetID   string
	Main       bool
	GatewayIds []string
}

func MockDescribeRouteTableOutput(mockedRouteTables ...TestRouteTable) *ec2.DescribeRouteTablesOutput {
	routeTables := make([]types.RouteTable, 0, len(mockedRouteTables))
	for _, mrt := range mockedRouteTables {
		routes := make([]types.Route, 0, len(mrt.GatewayIds))
		for _, gwID := range mrt.GatewayIds {
			routes = append(routes, types.Route{GatewayId: aws.String(gwID)})
		}
		rt := types.RouteTable{
			Associations: []types.RouteTableAssociation{
				{SubnetId: aws.String(mrt.SubnetID), Main: aws.Bool(mrt.Main)},
			},
			Routes: routes,
		}
		routeTables = append(routeTables, rt)
	}
	return &ec2.DescribeRouteTablesOutput{RouteTables: routeTables}
}
