package mock

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/zalando-incubator/kube-ingress-aws-controller/aws"

	"github.com/stretchr/testify/mock"
)

// AutoScalingAPI is a mock implementation of [aws.AutoScalingAPI]
type AutoScalingAPI struct {
	mock.Mock
}

var _ aws.AutoScalingAPI = &AutoScalingAPI{}

func (m *AutoScalingAPI) DescribeAutoScalingGroups(ctx context.Context, params *autoscaling.DescribeAutoScalingGroupsInput, optFns ...func(*autoscaling.Options)) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*autoscaling.DescribeAutoScalingGroupsOutput), args.Error(1)
}

func (m *AutoScalingAPI) DescribeLoadBalancerTargetGroups(ctx context.Context, params *autoscaling.DescribeLoadBalancerTargetGroupsInput, optFns ...func(*autoscaling.Options)) (*autoscaling.DescribeLoadBalancerTargetGroupsOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*autoscaling.DescribeLoadBalancerTargetGroupsOutput), args.Error(1)
}

func (m *AutoScalingAPI) AttachLoadBalancerTargetGroups(ctx context.Context, params *autoscaling.AttachLoadBalancerTargetGroupsInput, optFns ...func(*autoscaling.Options)) (*autoscaling.AttachLoadBalancerTargetGroupsOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*autoscaling.AttachLoadBalancerTargetGroupsOutput), args.Error(1)
}

func (m *AutoScalingAPI) DetachLoadBalancerTargetGroups(ctx context.Context, params *autoscaling.DetachLoadBalancerTargetGroupsInput, optFns ...func(*autoscaling.Options)) (*autoscaling.DetachLoadBalancerTargetGroupsOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*autoscaling.DetachLoadBalancerTargetGroupsOutput), args.Error(1)
}

// CloudFormationAPI is a mock implementation of [aws.CloudFormationAPI]
type CloudFormationAPI struct {
	mock.Mock
}

var _ aws.CloudFormationAPI = &CloudFormationAPI{}

func (m *CloudFormationAPI) DescribeStacks(ctx context.Context, params *cloudformation.DescribeStacksInput, optFns ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*cloudformation.DescribeStacksOutput), args.Error(1)
}

func (m *CloudFormationAPI) CreateStack(ctx context.Context, params *cloudformation.CreateStackInput, optFns ...func(*cloudformation.Options)) (*cloudformation.CreateStackOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*cloudformation.CreateStackOutput), args.Error(1)
}

func (m *CloudFormationAPI) UpdateTerminationProtection(ctx context.Context, params *cloudformation.UpdateTerminationProtectionInput, optFns ...func(*cloudformation.Options)) (*cloudformation.UpdateTerminationProtectionOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*cloudformation.UpdateTerminationProtectionOutput), args.Error(1)
}

func (m *CloudFormationAPI) UpdateStack(ctx context.Context, params *cloudformation.UpdateStackInput, optFns ...func(*cloudformation.Options)) (*cloudformation.UpdateStackOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*cloudformation.UpdateStackOutput), args.Error(1)
}

func (m *CloudFormationAPI) DeleteStack(ctx context.Context, params *cloudformation.DeleteStackInput, optFns ...func(*cloudformation.Options)) (*cloudformation.DeleteStackOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*cloudformation.DeleteStackOutput), args.Error(1)
}

// EC2API is a mock implementation of [aws.EC2API]
type EC2API struct {
	mock.Mock
}

var _ aws.EC2API = &EC2API{}

func (m *EC2API) DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*ec2.DescribeInstancesOutput), args.Error(1)
}

func (m *EC2API) DescribeSubnets(ctx context.Context, params *ec2.DescribeSubnetsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*ec2.DescribeSubnetsOutput), args.Error(1)
}

func (m *EC2API) DescribeRouteTables(ctx context.Context, params *ec2.DescribeRouteTablesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeRouteTablesOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*ec2.DescribeRouteTablesOutput), args.Error(1)
}

func (m *EC2API) DescribeSecurityGroups(ctx context.Context, params *ec2.DescribeSecurityGroupsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*ec2.DescribeSecurityGroupsOutput), args.Error(1)
}

// ELBV2API is a mock implementation of [aws.ELBV2API]
type ELBV2API struct {
	mock.Mock
}

var _ aws.ELBV2API = &ELBV2API{}

func (m *ELBV2API) DescribeTargetGroups(ctx context.Context, params *elbv2.DescribeTargetGroupsInput, optFns ...func(*elbv2.Options)) (*elbv2.DescribeTargetGroupsOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*elbv2.DescribeTargetGroupsOutput), args.Error(1)
}

func (m *ELBV2API) DescribeTargetHealth(ctx context.Context, params *elbv2.DescribeTargetHealthInput, optFns ...func(*elbv2.Options)) (*elbv2.DescribeTargetHealthOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*elbv2.DescribeTargetHealthOutput), args.Error(1)
}

func (m *ELBV2API) DescribeLoadBalancers(ctx context.Context, params *elbv2.DescribeLoadBalancersInput, optFns ...func(*elbv2.Options)) (*elbv2.DescribeLoadBalancersOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*elbv2.DescribeLoadBalancersOutput), args.Error(1)
}

func (m *ELBV2API) DescribeTags(ctx context.Context, params *elbv2.DescribeTagsInput, optFns ...func(*elbv2.Options)) (*elbv2.DescribeTagsOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*elbv2.DescribeTagsOutput), args.Error(1)
}

func (m *ELBV2API) RegisterTargets(ctx context.Context, params *elbv2.RegisterTargetsInput, optFns ...func(*elbv2.Options)) (*elbv2.RegisterTargetsOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*elbv2.RegisterTargetsOutput), args.Error(1)
}

func (m *ELBV2API) DeregisterTargets(ctx context.Context, params *elbv2.DeregisterTargetsInput, optFns ...func(*elbv2.Options)) (*elbv2.DeregisterTargetsOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*elbv2.DeregisterTargetsOutput), args.Error(1)
}
