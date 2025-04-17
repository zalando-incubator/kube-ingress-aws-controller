package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
)

type ELBV2IFaceAPI interface {
	DescribeTags(context.Context, *elbv2.DescribeTagsInput, ...func(*elbv2.Options)) (*elbv2.DescribeTagsOutput, error)
	DescribeTargetGroups(context.Context, *elbv2.DescribeTargetGroupsInput, ...func(*elbv2.Options)) (*elbv2.DescribeTargetGroupsOutput, error)
	RegisterTargets(context.Context, *elbv2.RegisterTargetsInput, ...func(*elbv2.Options)) (*elbv2.RegisterTargetsOutput, error)
	DeregisterTargets(context.Context, *elbv2.DeregisterTargetsInput, ...func(*elbv2.Options)) (*elbv2.DeregisterTargetsOutput, error)
	DescribeTargetHealth(context.Context, *elbv2.DescribeTargetHealthInput, ...func(*elbv2.Options)) (*elbv2.DescribeTargetHealthOutput, error)
}

func registerTargetsOnTargetGroups(ctx context.Context, svc ELBV2IFaceAPI, targetGroupARNs []string, instances []string) error {
	targets := make([]elbv2types.TargetDescription, len(instances))
	for i, instance := range instances {
		targets[i] = elbv2types.TargetDescription{
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

func deregisterTargetsOnTargetGroups(ctx context.Context, svc ELBV2IFaceAPI, targetGroupARNs []string, instances []string) error {
	targets := make([]elbv2types.TargetDescription, len(instances))
	for i, instance := range instances {
		targets[i] = elbv2types.TargetDescription{
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
