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

func getAutoScalingGroupsByName(service autoscalingiface.AutoScalingAPI, autoScalingGroupNames []string) (map[string]*autoScalingGroupDetails, error) {
	params := &autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: aws.StringSlice(autoScalingGroupNames),
	}
	resp, err := service.DescribeAutoScalingGroups(params)
	if err != nil {
		return nil, err
	}

	result := make(map[string]*autoScalingGroupDetails)
	for _, g := range resp.AutoScalingGroups {
		name := aws.StringValue(g.AutoScalingGroupName)
		tags := make(map[string]string)
		for _, td := range g.Tags {
			tags[aws.StringValue(td.Key)] = aws.StringValue(td.Value)
		}
		result[name] = &autoScalingGroupDetails{
			name: name,
			arn:  aws.StringValue(g.AutoScalingGroupARN),
			launchConfigurationName: aws.StringValue(g.LaunchConfigurationName),
			targetGroups:            aws.StringValueSlice(g.TargetGroupARNs),
			tags:                    tags,
		}
	}

	for _, name := range autoScalingGroupNames {
		if _, ok := result[name]; !ok {
			return nil, fmt.Errorf("auto scaling group %q not found", name)
		}
	}

	return result, nil
}

func updateTargetGroupsForAutoScalingGroup(svc autoscalingiface.AutoScalingAPI, targetGroupARNs []string, autoScalingGroupName string) error {
	params := &autoscaling.DescribeLoadBalancerTargetGroupsInput{
		AutoScalingGroupName: aws.String(autoScalingGroupName),
	}

	resp, err := svc.DescribeLoadBalancerTargetGroups(params)
	if err != nil {
		return err
	}

	// find obsolete target groups which should be detached
	detachARNs := make([]string, 0, len(targetGroupARNs))
	for _, tg := range resp.LoadBalancerTargetGroups {
		tgARN := aws.StringValue(tg.LoadBalancerTargetGroupARN)
		if !inStrSlice(tgARN, targetGroupARNs) {
			detachARNs = append(detachARNs, tgARN)
		}
	}

	attachParams := &autoscaling.AttachLoadBalancerTargetGroupsInput{
		AutoScalingGroupName: aws.String(autoScalingGroupName),
		TargetGroupARNs:      aws.StringSlice(targetGroupARNs),
	}
	_, err = svc.AttachLoadBalancerTargetGroups(attachParams)
	if err != nil {
		return err
	}

	if len(detachARNs) > 0 {
		err = detachTargetGroupsFromAutoScalingGroup(svc, detachARNs, autoScalingGroupName)
		if err != nil {
			return err
		}
	}

	return nil
}

func detachTargetGroupsFromAutoScalingGroup(svc autoscalingiface.AutoScalingAPI, targetGroupARNs []string, autoScalingGroupName string) error {
	params := &autoscaling.DetachLoadBalancerTargetGroupsInput{
		AutoScalingGroupName: aws.String(autoScalingGroupName),
		TargetGroupARNs:      aws.StringSlice(targetGroupARNs),
	}
	_, err := svc.DetachLoadBalancerTargetGroups(params)
	if err != nil {
		return err
	}

	return nil
}

func inStrSlice(item string, slice []string) bool {
	for _, str := range slice {
		if str == item {
			return true
		}
	}
	return false
}
