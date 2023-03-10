package fake

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/aws/aws-sdk-go/service/elbv2/elbv2iface"
)

type Elbv2MockOutputs struct {
	RegisterTargets      *ApiResponse
	DeregisterTargets    *ApiResponse
	DescribeTags         *ApiResponse
	DescribeTargetGroups *ApiResponse
	DescribeTargetHealth *ApiResponse
}

type MockElbv2Client struct {
	elbv2iface.ELBV2API
	Outputs  Elbv2MockOutputs
	Rtinputs []*elbv2.RegisterTargetsInput
	Dtinputs []*elbv2.DeregisterTargetsInput
}

func (m *MockElbv2Client) RegisterTargets(in *elbv2.RegisterTargetsInput) (*elbv2.RegisterTargetsOutput, error) {
	m.Rtinputs = append(m.Rtinputs, in)
	out, ok := m.Outputs.RegisterTargets.response.(*elbv2.RegisterTargetsOutput)
	if !ok {
		return nil, m.Outputs.RegisterTargets.err
	}
	return out, m.Outputs.RegisterTargets.err
}

func MockRTOutput() *elbv2.RegisterTargetsOutput {
	return &elbv2.RegisterTargetsOutput{}
}

func (m *MockElbv2Client) DeregisterTargets(in *elbv2.DeregisterTargetsInput) (*elbv2.DeregisterTargetsOutput, error) {
	m.Dtinputs = append(m.Dtinputs, in)
	out, ok := m.Outputs.DeregisterTargets.response.(*elbv2.DeregisterTargetsOutput)
	if !ok {
		return nil, m.Outputs.DeregisterTargets.err
	}
	return out, m.Outputs.DeregisterTargets.err
}

func (m *MockElbv2Client) DescribeTags(tags *elbv2.DescribeTagsInput) (*elbv2.DescribeTagsOutput, error) {
	out, ok := m.Outputs.DescribeTags.response.(*elbv2.DescribeTagsOutput)
	if !ok {
		return nil, m.Outputs.DescribeTags.err
	}
	return out, m.Outputs.DescribeTags.err
}

func (m *MockElbv2Client) DescribeTargetGroupsPagesWithContext(ctx aws.Context, in *elbv2.DescribeTargetGroupsInput, f func(resp *elbv2.DescribeTargetGroupsOutput, lastPage bool) bool, opt ...request.Option) error {
	if out, ok := m.Outputs.DescribeTargetGroups.response.(*elbv2.DescribeTargetGroupsOutput); ok {
		f(out, true)
	}
	return m.Outputs.DescribeTargetGroups.err
}

func (m *MockElbv2Client) DescribeTargetHealth(*elbv2.DescribeTargetHealthInput) (*elbv2.DescribeTargetHealthOutput, error) {
	out, ok := m.Outputs.DescribeTargetHealth.response.(*elbv2.DescribeTargetHealthOutput)
	if !ok {
		return nil, m.Outputs.DescribeTargetHealth.err
	}
	return out, m.Outputs.DescribeTargetHealth.err
}

func MockDeregisterTargetsOutput() *elbv2.DeregisterTargetsOutput {
	return &elbv2.DeregisterTargetsOutput{}
}
