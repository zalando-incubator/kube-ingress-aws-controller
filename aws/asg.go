package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/service/autoscaling"
)

func GetLoadBalancers(p client.ConfigProvider, autoScalingGroupName string) ([]string, error) {
	svc := autoscaling.New(p)

	params := &autoscaling.DescribeLoadBalancersInput{
		AutoScalingGroupName: aws.String(autoScalingGroupName),
		// max for classic - http://docs.aws.amazon.com/autoscaling/latest/userguide/as-account-limits.html
		MaxRecords: aws.Int64(50),
	}
	resp, err := svc.DescribeLoadBalancers(params)

	if err != nil {
		return nil, err
	}

	lbs := make([]string, len(resp.LoadBalancers))
	for i, lb := range resp.LoadBalancers {
		lbs[i] = aws.StringValue(lb.LoadBalancerName)
	}
	return lbs, nil
}
