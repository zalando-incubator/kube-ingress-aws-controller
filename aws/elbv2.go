package aws

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/aws/aws-sdk-go/service/elbv2/elbv2iface"
)

func registerTargetsOnTargetGroups(svc elbv2iface.ELBV2API, targetGroupARNs []string, instances []string) error {
	targets := make([]*elbv2.TargetDescription, len(instances))
	for i, instance := range instances {
		targets[i] = &elbv2.TargetDescription{
			Id: aws.String(instance),
		}
	}

	for _, targetGroupARN := range targetGroupARNs {
		input := &elbv2.RegisterTargetsInput{
			TargetGroupArn: aws.String(targetGroupARN),
			Targets:        targets,
		}

		resp, err := svc.RegisterTargets(input)
		if err != nil || resp == nil {
			return fmt.Errorf("unable to register instances %q in target group %s: %v", instances, targetGroupARN, err)
		}
	}
	return nil
}
