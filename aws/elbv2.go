package aws

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/aws/aws-sdk-go-v2/aws"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2Types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
)

type LoadBalancerState struct {
	StateCode elbv2Types.LoadBalancerStateEnum
	Reason    string
}

type StackLBState struct {
	Stack   *Stack
	LBState *LoadBalancerState
}

type ELBV2API interface {
	elbv2.DescribeTargetGroupsAPIClient
	elbv2.DescribeTargetHealthAPIClient
	elbv2.DescribeLoadBalancersAPIClient
	DescribeTags(context.Context, *elbv2.DescribeTagsInput, ...func(*elbv2.Options)) (*elbv2.DescribeTagsOutput, error)
	RegisterTargets(context.Context, *elbv2.RegisterTargetsInput, ...func(*elbv2.Options)) (*elbv2.RegisterTargetsOutput, error)
	DeregisterTargets(context.Context, *elbv2.DeregisterTargetsInput, ...func(*elbv2.Options)) (*elbv2.DeregisterTargetsOutput, error)
}

// IsActiveLBState checks if the LoadBalancerState is in an active state. Return
// false if the LoadBalancerState is nil.
func IsActiveLBState(ls *LoadBalancerState) bool {
	return ls != nil && ls.StateCode == elbv2Types.LoadBalancerStateEnumActive
}

// GetLBStateString returns a string representation of the LoadBalancerState.
func GetLBStateString(ls *LoadBalancerState) string {
	if ls == nil {
		return "nil"
	}
	return string(ls.StateCode)
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

func getLoadBalancerStates(
	ctx context.Context,
	svc ELBV2API,
	loadBalancerARNs []string,
) (
	map[string]*LoadBalancerState,
	error,
) {
	loadBalancerStates := make(map[string]*LoadBalancerState, len(loadBalancerARNs))

	if len(loadBalancerARNs) == 0 {
		return loadBalancerStates, nil
	}

	// DescribeLoadBalancers API call's loadBalancerArns request parameter can only take 20 items at a time.
	// https://docs.aws.amazon.com/elasticloadbalancing/latest/APIReference/API_DescribeLoadBalancers.html
	const maxLoadBalancerARNsPerRequest = 20

	for i := 0; i < len(loadBalancerARNs); i += maxLoadBalancerARNsPerRequest {
		end := min(i+maxLoadBalancerARNsPerRequest, len(loadBalancerARNs))
		chunk := loadBalancerARNs[i:end]
		input := &elbv2.DescribeLoadBalancersInput{
			LoadBalancerArns: chunk,
		}
		output, err := svc.DescribeLoadBalancers(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("unable to describe load balancers %v: %w", chunk, err)
		}
		for _, lb := range output.LoadBalancers {
			if lb.State == nil {
				log.Warnf("Loadbalancer %s has no state information", aws.ToString(lb.LoadBalancerArn))
				loadBalancerStates[aws.ToString(lb.LoadBalancerArn)] = nil
			} else {
				loadBalancerStates[aws.ToString(lb.LoadBalancerArn)] = &LoadBalancerState{
					StateCode: lb.State.Code,
					Reason:    aws.ToString(lb.State.Reason),
				}
			}

		}
	}
	return loadBalancerStates, nil
}
