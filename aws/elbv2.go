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

		_, err := svc.RegisterTargets(input)
		if err != nil {
			return fmt.Errorf("unable to register instances %q in target group %s: %v", instances, targetGroupARN, err)
		}
	}
	return nil
}

func deregisterTargetsOnTargetGroups(svc elbv2iface.ELBV2API, targetGroupARNs []string, instances []string) error {
	targets := make([]*elbv2.TargetDescription, len(instances))
	for i, instance := range instances {
		targets[i] = &elbv2.TargetDescription{
			Id: aws.String(instance),
		}
	}

	for _, targetGroupARN := range targetGroupARNs {
		input := &elbv2.DeregisterTargetsInput{
			TargetGroupArn: aws.String(targetGroupARN),
			Targets:        targets,
		}

		_, err := svc.DeregisterTargets(input)
		if err != nil {
			return fmt.Errorf("unable to deregister instances %q in target group %s: %v", instances, targetGroupARN, err)
		}
	}
	return nil
}
