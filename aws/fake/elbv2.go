package fake

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2Types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
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

func (m *ELBv2Client) DescribeLoadBalancers(ctx context.Context, input *elbv2.DescribeLoadBalancersInput, opts ...func(*elbv2.Options)) (*elbv2.DescribeLoadBalancersOutput, error) {
	if input != nil && len(input.LoadBalancerArns) > 20 {
		return nil, fmt.Errorf("too many load balancers requested: %d, maximum is 20", len(input.LoadBalancerArns))
	}

	// if mocked output is provided, return the mocked output
	if m.Outputs.DescribeLoadBalancers != nil {
		out, ok := m.Outputs.DescribeLoadBalancers.response.(*elbv2.DescribeLoadBalancersOutput)
		if !ok {
			return nil, m.Outputs.DescribeLoadBalancers.err
		}
		if out == nil {
			return &elbv2.DescribeLoadBalancersOutput{}, m.Outputs.DescribeLoadBalancers.err
		}
		return out, m.Outputs.DescribeLoadBalancers.err
	}

	// if no mocked output is provided, create the output based on the input
	lbs := make([]elbv2Types.LoadBalancer, len(input.LoadBalancerArns))
	for i, arn := range input.LoadBalancerArns {
		lbs[i] = elbv2Types.LoadBalancer{
			LoadBalancerArn: &arn,
			State: &elbv2Types.LoadBalancerState{
				Code:   elbv2Types.LoadBalancerStateEnumActive,
				Reason: aws.String("Mocked state"),
			},
		}
	}
	return &elbv2.DescribeLoadBalancersOutput{LoadBalancers: lbs}, nil
}

func MockDescribeTargetGroupsOutput() *elbv2.DescribeTargetGroupsOutput {
	return &elbv2.DescribeTargetGroupsOutput{}
}

func MockDeregisterTargetsOutput() *elbv2.DeregisterTargetsOutput {
	return &elbv2.DeregisterTargetsOutput{}
}

func MockDescribeLoadBalancersOutput() *elbv2.DescribeLoadBalancersOutput {
	return &elbv2.DescribeLoadBalancersOutput{}
}
