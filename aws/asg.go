package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/aws/aws-sdk-go/service/elbv2/elbv2iface"

	log "github.com/sirupsen/logrus"
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
				name:                    autoScalingGroupName,
				arn:                     aws.StringValue(g.AutoScalingGroupARN),
				targetGroups:            aws.StringValueSlice(g.TargetGroupARNs),
				launchConfigurationName: aws.StringValue(g.LaunchConfigurationName),
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
			name:                    name,
			arn:                     aws.StringValue(g.AutoScalingGroupARN),
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

// Given a set of filter tags, and actual ASG tags, iterate over every filter tag,
// looking for a matching tag name on the ASG. If one is seen, and our filter value is
// empty, or contains the value on the ASG tag, count it as a match. If all matched,
// Test as true, otherwise return false
func testFilterTags(filterTags map[string][]string, asgTags map[string]string) bool {
	matches := make(map[string]int)
	for filterKey, filterValues := range filterTags {
		if v, found := asgTags[filterKey]; found {
			if len(filterValues) == 0 {
				matches[filterKey] = matches[filterKey] + 1
			} else {
				for _, filterVal := range filterValues {
					if v == filterVal {
						matches[filterKey] = matches[filterKey] + 1
					}
				}
			}
		} else {
			// failed to match, return fast
			return false
		}
	}
	if len(filterTags) == len(matches) {
		return true
	}
	return false
}

func getOwnedAndTargetedAutoScalingGroups(service autoscalingiface.AutoScalingAPI, filterTags map[string][]string, ownedTags map[string]string) (map[string]*autoScalingGroupDetails, map[string]*autoScalingGroupDetails, error) {
	params := &autoscaling.DescribeAutoScalingGroupsInput{}

	targetedASGs := make(map[string]*autoScalingGroupDetails)
	ownedASGs := make(map[string]*autoScalingGroupDetails)
	err := service.DescribeAutoScalingGroupsPages(params,
		func(page *autoscaling.DescribeAutoScalingGroupsOutput, lastPage bool) bool {
			for _, g := range page.AutoScalingGroups {
				name := aws.StringValue(g.AutoScalingGroupName)
				tags := make(map[string]string)
				for _, td := range g.Tags {
					key := aws.StringValue(td.Key)
					value := aws.StringValue(td.Value)
					tags[key] = value
				}

				asg := &autoScalingGroupDetails{
					name:                    name,
					arn:                     aws.StringValue(g.AutoScalingGroupARN),
					launchConfigurationName: aws.StringValue(g.LaunchConfigurationName),
					targetGroups:            aws.StringValueSlice(g.TargetGroupARNs),
					tags:                    tags,
				}

				if hasTagsASG(g.Tags, ownedTags) {
					ownedASGs[name] = asg
				}

				if testFilterTags(filterTags, tags) {
					targetedASGs[name] = asg
				}
			}

			return true
		})
	if err != nil {
		return nil, nil, err
	}

	return targetedASGs, ownedASGs, nil
}

func updateTargetGroupsForAutoScalingGroup(svc autoscalingiface.AutoScalingAPI, elbv2svc elbv2iface.ELBV2API, targetGroupARNs []string, autoScalingGroupName string, ownerTags map[string]string) error {
	params := &autoscaling.DescribeLoadBalancerTargetGroupsInput{
		AutoScalingGroupName: aws.String(autoScalingGroupName),
	}

	resp, err := svc.DescribeLoadBalancerTargetGroups(params)
	if err != nil {
		return err
	}

	allTGs, err := describeTargetGroups(elbv2svc)
	if err != nil {
		return err
	}

	if len(resp.LoadBalancerTargetGroups) > 0 {
		// find non-existing target groups which should be detached
		detachARNs := make([]string, 0, len(resp.LoadBalancerTargetGroups))
		validARNs := make([]string, 0, len(resp.LoadBalancerTargetGroups))
		for _, tg := range resp.LoadBalancerTargetGroups {
			tgARN := aws.StringValue(tg.LoadBalancerTargetGroupARN)

			// check that TG exists at all, otherwise detach it
			if _, ok := allTGs[tgARN]; !ok {
				detachARNs = append(detachARNs, tgARN)
			} else {
				validARNs = append(validARNs, tgARN)
			}
		}

		descs, err := describeTags(elbv2svc, validARNs)
		if err != nil {
			return err
		}

		// find obsolete target groups which should be detached
		for _, tgARN := range validARNs {
			// only consider detaching TGs which are owned by the
			// controller
			if !tgHasTags(descs, tgARN, ownerTags) {
				continue
			}

			if !inStrSlice(tgARN, targetGroupARNs) {
				detachARNs = append(detachARNs, tgARN)
			}
		}

		if len(detachARNs) > 0 {
			err := detachTargetGroupsFromAutoScalingGroup(svc, detachARNs, autoScalingGroupName)
			if err != nil {
				return err
			}
		}
	}

	attachARNs := make([]string, 0, len(targetGroupARNs))
	for _, tgARN := range targetGroupARNs {
		if _, ok := allTGs[tgARN]; ok {
			attachARNs = append(attachARNs, tgARN)
		} else {
			// TODO: it is better to validate stack's target groups earlier to identify owning stack
			log.Errorf("Target group %q does not exist, will not attach", tgARN)
		}
	}

	if len(attachARNs) > 0 {
		err := attachTargetGroupsToAutoScalingGroup(svc, attachARNs, autoScalingGroupName)
		if err != nil {
			return err
		}
	}

	return nil
}

func describeTags(svc elbv2iface.ELBV2API, arns []string) ([]*elbv2.TagDescription, error) {
	descs := make([]*elbv2.TagDescription, 0, len(arns))
	// You can specify up to 20 resources in a single call,
	// see https://docs.aws.amazon.com/sdk-for-go/api/service/elbv2/#DescribeTagsInput
	err := processChunked(arns, 20, func(chunk []string) error {
		tgResp, err := svc.DescribeTags(&elbv2.DescribeTagsInput{
			ResourceArns: aws.StringSlice(chunk),
		})
		if err == nil {
			descs = append(descs, tgResp.TagDescriptions...)
		}
		return err
	})
	return descs, err
}

func describeTargetGroups(elbv2svc elbv2iface.ELBV2API) (map[string]struct{}, error) {
	targetGroups := make(map[string]struct{})
	err := elbv2svc.DescribeTargetGroupsPagesWithContext(context.TODO(), &elbv2.DescribeTargetGroupsInput{}, func(resp *elbv2.DescribeTargetGroupsOutput, lastPage bool) bool {
		for _, tg := range resp.TargetGroups {
			targetGroups[aws.StringValue(tg.TargetGroupArn)] = struct{}{}
		}
		return true
	})
	return targetGroups, err
}

// tgHasTags returns true if the specified resource has the expected tags.
func tgHasTags(descs []*elbv2.TagDescription, arn string, tags map[string]string) bool {
	for _, desc := range descs {
		if aws.StringValue(desc.ResourceArn) == arn && hasTags(desc.Tags, tags) {
			return true
		}
	}
	return false
}

// hasTags returns true if the expectedTags are found in the list of tags.
func hasTags(tags []*elbv2.Tag, expectedTags map[string]string) bool {
	for key, val := range expectedTags {
		found := false
		for _, tag := range tags {
			if aws.StringValue(tag.Key) == key && aws.StringValue(tag.Value) == val {
				found = true
				break
			}
		}

		if !found {
			return false
		}
	}
	return true
}

// hasTagsASG returns true if the expectedTags are found in the list of tags.
func hasTagsASG(tags []*autoscaling.TagDescription, expectedTags map[string]string) bool {
	for key, val := range expectedTags {
		found := false
		for _, tag := range tags {
			if aws.StringValue(tag.Key) == key && aws.StringValue(tag.Value) == val {
				found = true
				break
			}
		}

		if !found {
			return false
		}
	}
	return true
}

func attachTargetGroupsToAutoScalingGroup(svc autoscalingiface.AutoScalingAPI, targetGroupARNs []string, autoScalingGroupName string) error {
	// You can specify up to 10 target groups,
	// see https://docs.aws.amazon.com/sdk-for-go/api/service/autoscaling/#AttachLoadBalancerTargetGroupsInput
	return processChunked(targetGroupARNs, 10, func(chunk []string) error {
		_, err := svc.AttachLoadBalancerTargetGroups(&autoscaling.AttachLoadBalancerTargetGroupsInput{
			AutoScalingGroupName: aws.String(autoScalingGroupName),
			TargetGroupARNs:      aws.StringSlice(chunk),
		})
		return err
	})
}

func detachTargetGroupsFromAutoScalingGroup(svc autoscalingiface.AutoScalingAPI, targetGroupARNs []string, autoScalingGroupName string) error {
	// You can specify up to 10 target groups,
	// see https://docs.aws.amazon.com/sdk-for-go/api/service/autoscaling/#DetachLoadBalancerTargetGroupsInput
	return processChunked(targetGroupARNs, 10, func(chunk []string) error {
		_, err := svc.DetachLoadBalancerTargetGroups(&autoscaling.DetachLoadBalancerTargetGroupsInput{
			AutoScalingGroupName: aws.String(autoScalingGroupName),
			TargetGroupARNs:      aws.StringSlice(chunk),
		})
		return err
	})
}

// Processes slice in chunks
// Stops and returns on the first error encountered
func processChunked(slice []string, chunkSize int, process func(chunk []string) error) error {
	for i := 0; i < len(slice); i += chunkSize {
		end := i + chunkSize

		if end > len(slice) {
			end = len(slice)
		}

		chunk := slice[i:end]
		if len(chunk) > 0 {
			if err := process(chunk); err != nil {
				return err
			}
		}
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
