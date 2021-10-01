package aws

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
)

type autoscalingMockInputs struct {
	attachLoadBalancerTargetGroups func(*testing.T, *autoscaling.AttachLoadBalancerTargetGroupsInput)
}

type autoscalingMockOutputs struct {
	describeAutoScalingGroups        *apiResponse
	attachLoadBalancerTargetGroups   *apiResponse
	detachLoadBalancerTargetGroups   *apiResponse
	describeLoadBalancerTargetGroups *apiResponse
}

type mockAutoScalingClient struct {
	autoscalingiface.AutoScalingAPI
	outputs autoscalingMockOutputs
	inputs  autoscalingMockInputs
	t       *testing.T
}

func (m *mockAutoScalingClient) DescribeAutoScalingGroups(*autoscaling.DescribeAutoScalingGroupsInput) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
	if out, ok := m.outputs.describeAutoScalingGroups.response.(*autoscaling.DescribeAutoScalingGroupsOutput); ok {
		return out, m.outputs.describeAutoScalingGroups.err
	}
	return nil, m.outputs.describeAutoScalingGroups.err
}

func (m *mockAutoScalingClient) DescribeAutoScalingGroupsPages(_ *autoscaling.DescribeAutoScalingGroupsInput, fn func(*autoscaling.DescribeAutoScalingGroupsOutput, bool) bool) error {
	if out, ok := m.outputs.describeAutoScalingGroups.response.(*autoscaling.DescribeAutoScalingGroupsOutput); ok {
		fn(out, true)
	}
	return m.outputs.describeAutoScalingGroups.err
}

func (m *mockAutoScalingClient) DescribeLoadBalancerTargetGroups(*autoscaling.DescribeLoadBalancerTargetGroupsInput) (*autoscaling.DescribeLoadBalancerTargetGroupsOutput, error) {
	if out, ok := m.outputs.describeLoadBalancerTargetGroups.response.(*autoscaling.DescribeLoadBalancerTargetGroupsOutput); ok {
		return out, m.outputs.describeLoadBalancerTargetGroups.err
	}
	return nil, m.outputs.describeLoadBalancerTargetGroups.err
}

func (m *mockAutoScalingClient) AttachLoadBalancerTargetGroups(input *autoscaling.AttachLoadBalancerTargetGroupsInput) (*autoscaling.AttachLoadBalancerTargetGroupsOutput, error) {
	if m.inputs.attachLoadBalancerTargetGroups != nil {
		m.inputs.attachLoadBalancerTargetGroups(m.t, input)
	}
	return nil, m.outputs.attachLoadBalancerTargetGroups.err
}

func (m *mockAutoScalingClient) DetachLoadBalancerTargetGroups(*autoscaling.DetachLoadBalancerTargetGroupsInput) (*autoscaling.DetachLoadBalancerTargetGroupsOutput, error) {
	return nil, m.outputs.detachLoadBalancerTargetGroups.err
}

func mockDASGOutput(asgs ...map[string]asgtags) *autoscaling.DescribeAutoScalingGroupsOutput {
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
