package aws

import (
	"errors"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/service/ec2"
)

const autoScalingGroupNameTag = "aws:autoscaling:groupName"

var (
	ErrMissingAutoScalingGroupTag = errors.New(`instance is missing the "` + autoScalingGroupNameTag + `" tag`)
)

func GetAutoScalingGroupName(p client.ConfigProvider) (string, error) {
	svc := ec2.New(p)

	params := &ec2.DescribeTagsInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("key"),
				Values: []*string{
					aws.String(autoScalingGroupNameTag),
				},
			},
		},
	}
	resp, err := svc.DescribeTags(params)

	if err != nil {
		return "", err
	}

	if len(resp.Tags) < 1 {
		return "", ErrMissingAutoScalingGroupTag
	}

	return aws.StringValue(resp.Tags[0].Value), nil
}
