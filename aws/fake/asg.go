package fake

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
)

type ASGtags map[string]string

type ASGInputs struct {
	AttachLoadBalancerTargetGroups func(*testing.T, *autoscaling.AttachLoadBalancerTargetGroupsInput)
}

type ASGOutputs struct {
	DescribeAutoScalingGroups        *APIResponse
	AttachLoadBalancerTargetGroups   *APIResponse
	DetachLoadBalancerTargetGroups   *APIResponse
	DescribeLoadBalancerTargetGroups *APIResponse
}

type ASGClient struct {
	Outputs ASGOutputs
	Inputs  ASGInputs
	T       *testing.T
}

func (m *ASGClient) DescribeAutoScalingGroups(context.Context, *autoscaling.DescribeAutoScalingGroupsInput, ...func(*autoscaling.Options)) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
	if out, ok := m.Outputs.DescribeAutoScalingGroups.response.(*autoscaling.DescribeAutoScalingGroupsOutput); ok {
		return out, m.Outputs.DescribeAutoScalingGroups.err
	}

	return nil, m.Outputs.DescribeAutoScalingGroups.err
}

func (m *ASGClient) DescribeLoadBalancerTargetGroups(context.Context, *autoscaling.DescribeLoadBalancerTargetGroupsInput, ...func(*autoscaling.Options)) (*autoscaling.DescribeLoadBalancerTargetGroupsOutput, error) {
	if out, ok := m.Outputs.DescribeLoadBalancerTargetGroups.response.(*autoscaling.DescribeLoadBalancerTargetGroupsOutput); ok {
		return out, m.Outputs.DescribeLoadBalancerTargetGroups.err
	}
	return nil, m.Outputs.DescribeLoadBalancerTargetGroups.err
}

func (m *ASGClient) AttachLoadBalancerTargetGroups(ctx context.Context, input *autoscaling.AttachLoadBalancerTargetGroupsInput, opts ...func(*autoscaling.Options)) (*autoscaling.AttachLoadBalancerTargetGroupsOutput, error) {
	if m.Inputs.AttachLoadBalancerTargetGroups != nil {
		m.Inputs.AttachLoadBalancerTargetGroups(m.T, input)
	}
	return nil, m.Outputs.AttachLoadBalancerTargetGroups.err
}

func (m *ASGClient) DetachLoadBalancerTargetGroups(context.Context, *autoscaling.DetachLoadBalancerTargetGroupsInput, ...func(*autoscaling.Options)) (*autoscaling.DetachLoadBalancerTargetGroupsOutput, error) {
	return nil, m.Outputs.DetachLoadBalancerTargetGroups.err
}

func MockDescribeAutoScalingGroupOutput(asgs ...map[string]ASGtags) *autoscaling.DescribeAutoScalingGroupsOutput {
	groups := make([]types.AutoScalingGroup, 0)
	for _, asg := range asgs {
		for name, tags := range asg {
			t := make([]types.TagDescription, 0)
			for k, v := range tags {
				t = append(t, types.TagDescription{
					Key:   aws.String(k),
					Value: aws.String(v),
				})
			}
			g := types.AutoScalingGroup{
				AutoScalingGroupName: aws.String(name),
				Tags:                 t,
				TargetGroupARNs:      []string{},
			}
			groups = append(groups, g)
		}
	}
	return &autoscaling.DescribeAutoScalingGroupsOutput{AutoScalingGroups: groups}
}
