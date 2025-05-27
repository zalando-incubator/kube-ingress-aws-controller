package fake

import (
	"context"

	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
)

type ELBv2Outputs struct {
	RegisterTargets       *APIResponse
	DeregisterTargets     *APIResponse
	DescribeTags          *APIResponse
	DescribeTargetGroups  *APIResponse
	DescribeTargetHealth  *APIResponse
	DescribeLoadBalancers *APIResponse
}

type ELBv2Client struct {
	Outputs  ELBv2Outputs
	Rtinputs []*elbv2.RegisterTargetsInput
	Dtinputs []*elbv2.DeregisterTargetsInput
}

func (m *ELBv2Client) RegisterTargets(ctx context.Context, in *elbv2.RegisterTargetsInput, fn ...func(*elbv2.Options)) (*elbv2.RegisterTargetsOutput, error) {
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

func (m *ELBv2Client) DeregisterTargets(ctx context.Context, in *elbv2.DeregisterTargetsInput, fn ...func(*elbv2.Options)) (*elbv2.DeregisterTargetsOutput, error) {
	m.Dtinputs = append(m.Dtinputs, in)
	out, ok := m.Outputs.DeregisterTargets.response.(*elbv2.DeregisterTargetsOutput)
	if !ok {
		return nil, m.Outputs.DeregisterTargets.err
	}
	return out, m.Outputs.DeregisterTargets.err
}

func (m *ELBv2Client) DescribeTags(context.Context, *elbv2.DescribeTagsInput, ...func(*elbv2.Options)) (*elbv2.DescribeTagsOutput, error) {
	out, ok := m.Outputs.DescribeTags.response.(*elbv2.DescribeTagsOutput)
	if !ok {
		return nil, m.Outputs.DescribeTags.err
	}
	return out, m.Outputs.DescribeTags.err
}

func (m *ELBv2Client) DescribeTargetHealth(context.Context, *elbv2.DescribeTargetHealthInput, ...func(*elbv2.Options)) (*elbv2.DescribeTargetHealthOutput, error) {
	out, ok := m.Outputs.DescribeTargetHealth.response.(*elbv2.DescribeTargetHealthOutput)
	if !ok {
		return nil, m.Outputs.DescribeTargetHealth.err
	}
	return out, m.Outputs.DescribeTargetHealth.err
}

func (m *ELBv2Client) DescribeTargetGroups(context.Context, *elbv2.DescribeTargetGroupsInput, ...func(*elbv2.Options)) (*elbv2.DescribeTargetGroupsOutput, error) {
	out, ok := m.Outputs.DescribeTargetGroups.response.(*elbv2.DescribeTargetGroupsOutput)
	if !ok {
		return nil, m.Outputs.DescribeTargetGroups.err
	}
	return out, m.Outputs.DescribeTargetGroups.err
}

func (m *ELBv2Client) DescribeLoadBalancers(context.Context, *elbv2.DescribeLoadBalancersInput, ...func(*elbv2.Options)) (*elbv2.DescribeLoadBalancersOutput, error) {
	out, ok := m.Outputs.DescribeLoadBalancers.response.(*elbv2.DescribeLoadBalancersOutput)
	if !ok {
		return nil, m.Outputs.DescribeLoadBalancers.err
	}
	return out, m.Outputs.DescribeLoadBalancers.err
}

func MockDescribeTargetGroupsOutput() *elbv2.DescribeTargetGroupsOutput {
	return &elbv2.DescribeTargetGroupsOutput{}
}

func MockDeregisterTargetsOutput() *elbv2.DeregisterTargetsOutput {
	return &elbv2.DeregisterTargetsOutput{}
}
