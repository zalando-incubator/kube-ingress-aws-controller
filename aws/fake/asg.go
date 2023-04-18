package fake

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
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
	autoscalingiface.AutoScalingAPI
	Outputs ASGOutputs
	Inputs  ASGInputs
	T       *testing.T
}

func (m *ASGClient) DescribeAutoScalingGroups(*autoscaling.DescribeAutoScalingGroupsInput) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
	if out, ok := m.Outputs.DescribeAutoScalingGroups.response.(*autoscaling.DescribeAutoScalingGroupsOutput); ok {
		return out, m.Outputs.DescribeAutoScalingGroups.err
	}

	return nil, m.Outputs.DescribeAutoScalingGroups.err
}

func (m *ASGClient) DescribeAutoScalingGroupsPages(_ *autoscaling.DescribeAutoScalingGroupsInput, fn func(*autoscaling.DescribeAutoScalingGroupsOutput, bool) bool) error {
	if out, ok := m.Outputs.DescribeAutoScalingGroups.response.(*autoscaling.DescribeAutoScalingGroupsOutput); ok {
		fn(out, true)
	}
	return m.Outputs.DescribeAutoScalingGroups.err
}

func (m *ASGClient) DescribeLoadBalancerTargetGroups(*autoscaling.DescribeLoadBalancerTargetGroupsInput) (*autoscaling.DescribeLoadBalancerTargetGroupsOutput, error) {
	if out, ok := m.Outputs.DescribeLoadBalancerTargetGroups.response.(*autoscaling.DescribeLoadBalancerTargetGroupsOutput); ok {
		return out, m.Outputs.DescribeLoadBalancerTargetGroups.err
	}
	return nil, m.Outputs.DescribeLoadBalancerTargetGroups.err
}

func (m *ASGClient) AttachLoadBalancerTargetGroups(input *autoscaling.AttachLoadBalancerTargetGroupsInput) (*autoscaling.AttachLoadBalancerTargetGroupsOutput, error) {
	if m.Inputs.AttachLoadBalancerTargetGroups != nil {
		m.Inputs.AttachLoadBalancerTargetGroups(m.T, input)
	}
	return nil, m.Outputs.AttachLoadBalancerTargetGroups.err
}

func (m *ASGClient) DetachLoadBalancerTargetGroups(*autoscaling.DetachLoadBalancerTargetGroupsInput) (*autoscaling.DetachLoadBalancerTargetGroupsOutput, error) {
	return nil, m.Outputs.DetachLoadBalancerTargetGroups.err
}

func MockDescribeAutoScalingGroupOutput(asgs ...map[string]ASGtags) *autoscaling.DescribeAutoScalingGroupsOutput {
	groups := make([]*autoscaling.Group, 0)
	for _, asg := range asgs {
		for name, tags := range asg {
			t := make([]*autoscaling.TagDescription, 0)
			for k, v := range tags {
				t = append(t, &autoscaling.TagDescription{
					Key:   aws.String(k),
					Value: aws.String(v),
				})
			}
			g := &autoscaling.Group{
				AutoScalingGroupName: aws.String(name),
				Tags:                 t,
			}
			groups = append(groups, g)
		}
	}
	return &autoscaling.DescribeAutoScalingGroupsOutput{AutoScalingGroups: groups}
}
