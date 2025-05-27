package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2Types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
)

type StackELB struct {
	Stack *Stack
	ELB   *elbv2Types.LoadBalancer
}

type ELBV2API interface {
	elbv2.DescribeTargetGroupsAPIClient
	elbv2.DescribeTargetHealthAPIClient
	elbv2.DescribeLoadBalancersAPIClient
	DescribeTags(context.Context, *elbv2.DescribeTagsInput, ...func(*elbv2.Options)) (*elbv2.DescribeTagsOutput, error)
	RegisterTargets(context.Context, *elbv2.RegisterTargetsInput, ...func(*elbv2.Options)) (*elbv2.RegisterTargetsOutput, error)
	DeregisterTargets(context.Context, *elbv2.DeregisterTargetsInput, ...func(*elbv2.Options)) (*elbv2.DeregisterTargetsOutput, error)
}

func registerTargetsOnTargetGroups(ctx context.Context, svc ELBV2API, targetGroupARNs []string, instances []string) error {
	targets := make([]elbv2Types.TargetDescription, len(instances))
	for i, instance := range instances {
		targets[i] = elbv2Types.TargetDescription{
			Id: aws.String(instance),
		}
	}

	for _, targetGroupARN := range targetGroupARNs {
		input := &elbv2.RegisterTargetsInput{
			TargetGroupArn: aws.String(targetGroupARN),
			Targets:        targets,
		}

		_, err := svc.RegisterTargets(ctx, input)
		if err != nil {
			return fmt.Errorf("unable to register instances %q in target group %s: %w", instances, targetGroupARN, err)
		}
	}
	return nil
}

func deregisterTargetsOnTargetGroups(ctx context.Context, svc ELBV2API, targetGroupARNs []string, instances []string) error {
	targets := make([]elbv2Types.TargetDescription, len(instances))
	for i, instance := range instances {
		targets[i] = elbv2Types.TargetDescription{
			Id: aws.String(instance),
		}
	}

	for _, targetGroupARN := range targetGroupARNs {
		input := &elbv2.DeregisterTargetsInput{
			TargetGroupArn: aws.String(targetGroupARN),
			Targets:        targets,
		}

		_, err := svc.DeregisterTargets(ctx, input)
		if err != nil {
			return fmt.Errorf("unable to deregister instances %q in target group %s: %w", instances, targetGroupARN, err)
		}
	}
	return nil
}

// GetELBState returns the LoadBalancerState. Sometimes requests to AWS API can
// get throttled, and we will not be able to retrieve the ELBs even after
// retries. In such a case here we can have a nil ELB in the StackELB struct. In
// such a case should return nil for the ELBState too.
func (s *StackELB) GetELBState() *elbv2Types.LoadBalancerState {
	if s.ELB == nil {
		return nil
	}
	return s.ELB.State
}
