package aws

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
)

type autoScalingGroupDetails struct {
	name string
	arn  string
	// Contains the ARNs of the target groups associated with the auto scaling group
	targetGroups            []string
	launchConfigurationName string
	tags                    map[string]string
}

func getAutoScalingGroupByName(service autoscalingiface.AutoScalingAPI, autoScalingGroupName string) (*autoScalingGroupDetails, error) {
	params := &autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: []*string{
			aws.String(autoScalingGroupName),
		},
	}
	resp, err := service.DescribeAutoScalingGroups(params)

	if err != nil {
		return nil, err
	}

	for _, g := range resp.AutoScalingGroups {
		if aws.StringValue(g.AutoScalingGroupName) == autoScalingGroupName {
			tags := make(map[string]string)
			for _, td := range g.Tags {
				tags[aws.StringValue(td.Key)] = aws.StringValue(td.Value)
			}
			return &autoScalingGroupDetails{
				name: autoScalingGroupName,
				arn:  aws.StringValue(g.AutoScalingGroupARN),
				launchConfigurationName: aws.StringValue(g.LaunchConfigurationName),
				targetGroups:            aws.StringValueSlice(g.TargetGroupARNs),
				tags:                    tags,
			}, nil
		}
	}

	return nil, fmt.Errorf("auto scaling group %q not found", autoScalingGroupName)
}

func attachTargetGroupToAutoScalingGroup(svc autoscalingiface.AutoScalingAPI, targetGroupARN string, autoScalingGroupName string) error {
	params := &autoscaling.AttachLoadBalancerTargetGroupsInput{
		AutoScalingGroupName: aws.String(autoScalingGroupName),
		TargetGroupARNs:      aws.StringSlice([]string{targetGroupARN}),
	}
	_, err := svc.AttachLoadBalancerTargetGroups(params)
	if err != nil {
		return err
	}

	return nil
}

func detachTargetGroupFromAutoScalingGroup(svc autoscalingiface.AutoScalingAPI, targetGroupARN string, autoScalingGroupName string) error {
	params := &autoscaling.DetachLoadBalancerTargetGroupsInput{
		AutoScalingGroupName: aws.String(autoScalingGroupName),
		TargetGroupARNs:      aws.StringSlice([]string{targetGroupARN}),
	}
	_, err := svc.DetachLoadBalancerTargetGroups(params)
	if err != nil {
		return err
	}

	return nil
}
