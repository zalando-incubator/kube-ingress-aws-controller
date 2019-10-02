package aws

import (
	"context"
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/aws/aws-sdk-go/service/elbv2/elbv2iface"
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

// We look to the same string used for instance filtering, however, we are much more limited in what can be done for
// ASGs. As such, we instead build a map of tags to look for as we iterate over all ASGs in getOwnedAutoScalingGroups
func parseFilterTags(clusterId string) map[string][]string {
	if filter, ok := os.LookupEnv(customTagFilterEnvVarName); ok {
		terms := strings.Fields(filter)
		filterTags := make(map[string][]string)
		for _, term := range terms {
			parts := strings.Split(term, "=")
			if len(parts) != 2 {
				log.Errorf("failed parsing %s, falling back to default", customTagFilterEnvVarName)
				return generateDefaultFilterTags(clusterId)
			}
			if parts[0] == "tag-key" {
				filterTags[parts[1]] = []string{}
			} else if strings.HasPrefix(parts[0], "tag:") {
				tagparts := strings.Split(parts[0], ":")
				filterTags[tagparts[1]] = strings.Split(parts[1], ",")
			} else {
				filterTags[parts[0]] = strings.Split(parts[1], ",")
			}
		}
		return filterTags
	}
	return generateDefaultFilterTags(clusterId)

}

func generateDefaultFilterTags(clusterId string) map[string][]string {
	filterTags := make(map[string][]string)
	filterTags[clusterIDTagPrefix+clusterId] = []string{resourceLifecycleOwned}
	return filterTags
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

func getOwnedAutoScalingGroups(service autoscalingiface.AutoScalingAPI, clusterID string) (map[string]*autoScalingGroupDetails, error) {
	params := &autoscaling.DescribeAutoScalingGroupsInput{}

	filterTags := parseFilterTags(clusterID)

	result := make(map[string]*autoScalingGroupDetails)
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

				if testFilterTags(filterTags, tags) {
					result[name] = &autoScalingGroupDetails{
						name:                    name,
						arn:                     aws.StringValue(g.AutoScalingGroupARN),
						launchConfigurationName: aws.StringValue(g.LaunchConfigurationName),
						targetGroups:            aws.StringValueSlice(g.TargetGroupARNs),
						tags:                    tags,
					}
				}
			}

			return true
		})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func updateTargetGroupsForAutoScalingGroup(svc autoscalingiface.AutoScalingAPI, elbv2svc elbv2iface.ELBV2API, targetGroupARNs []string, autoScalingGroupName string, ownerTags map[string]string) error {
	params := &autoscaling.DescribeLoadBalancerTargetGroupsInput{
		AutoScalingGroupName: aws.String(autoScalingGroupName),
	}

	resp, err := svc.DescribeLoadBalancerTargetGroups(params)
	if err != nil {
		return err
	}

	// get all target groups to ensure we are only working with target
	// groups that still exists.
	tgParams := &elbv2.DescribeTargetGroupsInput{}
	allTGs := make([]*elbv2.TargetGroup, 0, len(resp.LoadBalancerTargetGroups))
	err = elbv2svc.DescribeTargetGroupsPagesWithContext(context.TODO(), tgParams, func(resp *elbv2.DescribeTargetGroupsOutput, lastPage bool) bool {
		allTGs = append(allTGs, resp.TargetGroups...)
		return true
	})
	if err != nil {
		return err
	}

	detachARNs := make([]string, 0, len(targetGroupARNs))
	if len(resp.LoadBalancerTargetGroups) > 0 {
		arns := make([]*string, 0, len(resp.LoadBalancerTargetGroups))
		for _, tg := range resp.LoadBalancerTargetGroups {
			arns = append(arns, tg.LoadBalancerTargetGroupARN)
		}

		tgParams := &elbv2.DescribeTagsInput{
			ResourceArns: arns,
		}

		tgResp, err := elbv2svc.DescribeTags(tgParams)
		if err != nil {
			return err
		}

		// find obsolete target groups which should be detached
		for _, tg := range resp.LoadBalancerTargetGroups {
			tgARN := aws.StringValue(tg.LoadBalancerTargetGroupARN)

			// check that TG exists at all, otherwise detach it
			if !tgExists(tgARN, allTGs) {
				detachARNs = append(detachARNs, tgARN)
				continue
			}

			// only consider detaching TGs which are owned by the
			// controller
			if !tgHasTags(tgResp.TagDescriptions, tgARN, ownerTags) {
				continue
			}

			if !inStrSlice(tgARN, targetGroupARNs) {
				detachARNs = append(detachARNs, tgARN)
			}
		}
	}

	// limit target group updates to 10 at a time since this is the limit
	// in AWS API.
	chunkSize := 10

	for i := 0; i < len(targetGroupARNs); i += chunkSize {
		end := i + chunkSize

		if end > len(targetGroupARNs) {
			end = len(targetGroupARNs)
		}

		groups := targetGroupARNs[i:end]
		if len(groups) > 0 {
			attachParams := &autoscaling.AttachLoadBalancerTargetGroupsInput{
				AutoScalingGroupName: aws.String(autoScalingGroupName),
				TargetGroupARNs:      aws.StringSlice(groups),
			}
			_, err = svc.AttachLoadBalancerTargetGroups(attachParams)
			if err != nil {
				return err
			}
		}
	}

	if len(detachARNs) > 0 {
		err = detachTargetGroupsFromAutoScalingGroup(svc, detachARNs, autoScalingGroupName)
		if err != nil {
			return err
		}
	}

	return nil
}

// tgExists returns true if the targetGroupARN is found in the list of
// targetGroups.
func tgExists(targetGroupARN string, targetGroups []*elbv2.TargetGroup) bool {
	for _, tg := range targetGroups {
		if aws.StringValue(tg.TargetGroupArn) == targetGroupARN {
			return true
		}
	}
	return false
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
