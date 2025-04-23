package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2Types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	log "github.com/sirupsen/logrus"
)

type AutoScalingIFaceAPI interface {
	DescribeAutoScalingGroups(context.Context, *autoscaling.DescribeAutoScalingGroupsInput, ...func(*autoscaling.Options)) (*autoscaling.DescribeAutoScalingGroupsOutput, error)
	DescribeLoadBalancerTargetGroups(context.Context, *autoscaling.DescribeLoadBalancerTargetGroupsInput, ...func(*autoscaling.Options)) (*autoscaling.DescribeLoadBalancerTargetGroupsOutput, error)
	AttachLoadBalancerTargetGroups(context.Context, *autoscaling.AttachLoadBalancerTargetGroupsInput, ...func(*autoscaling.Options)) (*autoscaling.AttachLoadBalancerTargetGroupsOutput, error)
	DetachLoadBalancerTargetGroups(context.Context, *autoscaling.DetachLoadBalancerTargetGroupsInput, ...func(*autoscaling.Options)) (*autoscaling.DetachLoadBalancerTargetGroupsOutput, error)
}

type autoScalingGroupDetails struct {
	name string
	arn  string
	// Contains the ARNs of the target groups associated with the auto scaling group
	targetGroups            []string
	launchConfigurationName string
	tags                    map[string]string
}

func getAutoScalingGroupByName(ctx context.Context, service AutoScalingIFaceAPI, autoScalingGroupName string) (*autoScalingGroupDetails, error) {
	params := &autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: []string{
			autoScalingGroupName,
		},
	}
	resp, err := service.DescribeAutoScalingGroups(ctx, params)
	if err != nil {
		return nil, err
	}

	for _, g := range resp.AutoScalingGroups {
		if aws.ToString(g.AutoScalingGroupName) == autoScalingGroupName {
			tags := make(map[string]string)
			for _, td := range g.Tags {
				tags[aws.ToString(td.Key)] = aws.ToString(td.Value)
			}
			return &autoScalingGroupDetails{
				name:                    autoScalingGroupName,
				arn:                     aws.ToString(g.AutoScalingGroupARN),
				targetGroups:            g.TargetGroupARNs,
				launchConfigurationName: aws.ToString(g.LaunchConfigurationName),
				tags:                    tags,
			}, nil
		}
	}

	return nil, fmt.Errorf("auto scaling group %q not found", autoScalingGroupName)
}

func getAutoScalingGroupsByName(ctx context.Context, service AutoScalingIFaceAPI, autoScalingGroupNames []string) (map[string]*autoScalingGroupDetails, error) {
	params := &autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: autoScalingGroupNames,
	}
	resp, err := service.DescribeAutoScalingGroups(ctx, params)
	if err != nil {
		return nil, err
	}

	result := make(map[string]*autoScalingGroupDetails)
	for _, g := range resp.AutoScalingGroups {
		name := aws.ToString(g.AutoScalingGroupName)
		tags := make(map[string]string)
		for _, td := range g.Tags {
			tags[aws.ToString(td.Key)] = aws.ToString(td.Value)
		}
		result[name] = &autoScalingGroupDetails{
			name:                    name,
			arn:                     aws.ToString(g.AutoScalingGroupARN),
			launchConfigurationName: aws.ToString(g.LaunchConfigurationName),
			targetGroups:            g.TargetGroupARNs,
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
	return len(filterTags) == len(matches)
}

func getOwnedAndTargetedAutoScalingGroups(ctx context.Context, service AutoScalingIFaceAPI, filterTags map[string][]string, ownedTags map[string]string) (map[string]*autoScalingGroupDetails, map[string]*autoScalingGroupDetails, error) {
	params := &autoscaling.DescribeAutoScalingGroupsInput{}
	targetedASGs := make(map[string]*autoScalingGroupDetails)
	ownedASGs := make(map[string]*autoScalingGroupDetails)
	paginator := autoscaling.NewDescribeAutoScalingGroupsPaginator(service, params)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, nil, err
		}

		for _, g := range page.AutoScalingGroups {
			name := aws.ToString(g.AutoScalingGroupName)
			tags := make(map[string]string)
			for _, td := range g.Tags {
				key := aws.ToString(td.Key)
				value := aws.ToString(td.Value)
				tags[key] = value
			}

			asg := &autoScalingGroupDetails{
				name:                    name,
				arn:                     aws.ToString(g.AutoScalingGroupARN),
				launchConfigurationName: aws.ToString(g.LaunchConfigurationName),
				targetGroups:            g.TargetGroupARNs,
				tags:                    tags,
			}

			if hasTagsASG(g.Tags, ownedTags) {
				ownedASGs[name] = asg
			}

			if testFilterTags(filterTags, tags) {
				targetedASGs[name] = asg
			}
		}
	}
	return targetedASGs, ownedASGs, nil
}

func updateTargetGroupsForAutoScalingGroup(ctx context.Context, svc AutoScalingIFaceAPI, elbv2svc ELBV2IFaceAPI, targetGroupARNs []string, autoScalingGroupName string, ownerTags map[string]string) error {
	params := &autoscaling.DescribeLoadBalancerTargetGroupsInput{
		AutoScalingGroupName: aws.String(autoScalingGroupName),
	}

	resp, err := svc.DescribeLoadBalancerTargetGroups(ctx, params)
	if err != nil {
		return err
	}

	allTGs, err := describeTargetGroups(ctx, elbv2svc)
	if err != nil {
		return err
	}

	if len(resp.LoadBalancerTargetGroups) > 0 {
		// find non-existing target groups which should be detached
		detachARNs := make([]string, 0, len(resp.LoadBalancerTargetGroups))
		validARNs := make([]string, 0, len(resp.LoadBalancerTargetGroups))
		for _, tg := range resp.LoadBalancerTargetGroups {
			tgARN := aws.ToString(tg.LoadBalancerTargetGroupARN)

			// check that TG exists at all, otherwise detach it
			if _, ok := allTGs[tgARN]; !ok {
				detachARNs = append(detachARNs, tgARN)
			} else {
				validARNs = append(validARNs, tgARN)
			}
		}

		descs, err := describeTags(ctx, elbv2svc, validARNs)
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
			err := detachTargetGroupsFromAutoScalingGroup(ctx, svc, detachARNs, autoScalingGroupName)
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
		err := attachTargetGroupsToAutoScalingGroup(ctx, svc, attachARNs, autoScalingGroupName)
		if err != nil {
			return err
		}
	}

	return nil
}

func describeTags(ctx context.Context, svc ELBV2IFaceAPI, arns []string) ([]*elbv2Types.TagDescription, error) {
	descs := make([]*elbv2Types.TagDescription, 0, len(arns))
	// You can specify up to 20 resources in a single call,
	// see https://docs.aws.amazon.com/sdk-for-go/api/service/elasticloadbalancingv2/#DescribeTagsInput
	err := processChunked(arns, 20, func(chunk []string) error {
		tgResp, err := svc.DescribeTags(ctx, &elbv2.DescribeTagsInput{
			ResourceArns: chunk,
		})
		if err == nil {
			for i := range tgResp.TagDescriptions {
				descs = append(descs, &tgResp.TagDescriptions[i])
			}
		}
		return err
	})
	return descs, err
}

func describeTargetGroups(ctx context.Context, elbv2svc ELBV2IFaceAPI) (map[string]struct{}, error) {
	targetGroups := make(map[string]struct{})
	paginator := elbv2.NewDescribeTargetGroupsPaginator(elbv2svc, &elbv2.DescribeTargetGroupsInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return targetGroups, err
		}
		for _, tg := range page.TargetGroups {
			targetGroups[aws.ToString(tg.TargetGroupArn)] = struct{}{}
		}
	}
	return targetGroups, nil
}

// map the target group slice into specific types such as instance, ip, etc
func categorizeTargetTypeInstance(ctx context.Context, elbv2svc ELBV2IFaceAPI, allTGARNs []string) (map[string][]string, error) {
	targetTypes := make(map[string][]string)
	paginator := elbv2.NewDescribeTargetGroupsPaginator(elbv2svc, &elbv2.DescribeTargetGroupsInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return targetTypes, err
		}
		for _, tg := range page.TargetGroups {
			for _, v := range allTGARNs {
				if v != aws.ToString(tg.TargetGroupArn) {
					continue
				}
				targetTypes[string(tg.TargetType)] = append(targetTypes[string(tg.TargetType)], aws.ToString(tg.TargetGroupArn))
			}
		}
	}
	log.Debugf("categorized target group arns: %#v", targetTypes)
	return targetTypes, nil
}

// tgHasTags returns true if the specified resource has the expected tags.
func tgHasTags(descs []*elbv2Types.TagDescription, arn string, tags map[string]string) bool {
	for _, desc := range descs {
		if aws.ToString(desc.ResourceArn) == arn && hasTags(desc.Tags, tags) {
			return true
		}
	}
	return false
}

// hasTags returns true if the expectedTags are found in the list of tags.
func hasTags(tags []elbv2Types.Tag, expectedTags map[string]string) bool {
	for key, val := range expectedTags {
		found := false
		for _, tag := range tags {
			if aws.ToString(tag.Key) == key && aws.ToString(tag.Value) == val {
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
func hasTagsASG(tags []types.TagDescription, expectedTags map[string]string) bool {
	for key, val := range expectedTags {
		found := false
		for _, tag := range tags {
			if aws.ToString(tag.Key) == key && aws.ToString(tag.Value) == val {
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

func attachTargetGroupsToAutoScalingGroup(ctx context.Context, svc AutoScalingIFaceAPI, targetGroupARNs []string, autoScalingGroupName string) error {
	// You can specify up to 10 target groups,
	// see https://docs.aws.amazon.com/sdk-for-go/api/service/autoscaling/#AttachLoadBalancerTargetGroupsInput
	return processChunked(targetGroupARNs, 10, func(chunk []string) error {
		_, err := svc.AttachLoadBalancerTargetGroups(ctx, &autoscaling.AttachLoadBalancerTargetGroupsInput{
			AutoScalingGroupName: aws.String(autoScalingGroupName),
			TargetGroupARNs:      chunk,
		})
		return err
	})
}

func detachTargetGroupsFromAutoScalingGroup(ctx context.Context, svc AutoScalingIFaceAPI, targetGroupARNs []string, autoScalingGroupName string) error {
	// You can specify up to 10 target groups,
	// see https://docs.aws.amazon.com/sdk-for-go/api/service/autoscaling/#DetachLoadBalancerTargetGroupsInput
	return processChunked(targetGroupARNs, 10, func(chunk []string) error {
		_, err := svc.DetachLoadBalancerTargetGroups(ctx, &autoscaling.DetachLoadBalancerTargetGroupsInput{
			AutoScalingGroupName: aws.String(autoScalingGroupName),
			TargetGroupARNs:      chunk,
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
