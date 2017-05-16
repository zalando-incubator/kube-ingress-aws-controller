//go:generate go run gencftemplate.go
package aws

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
	"time"
)

// Stack is a simple wrapper around a CloudFormation Stack.
type Stack struct {
	name           string
	dnsName        string
	targetGroupARN string
	certificateARN string
}

func (s *Stack) Name() string {
	return s.name
}

func (s *Stack) DNSName() string {
	return s.dnsName
}

func (s *Stack) CertificateARN() string {
	return s.certificateARN
}

func (s *Stack) TargetGroupARN() string {
	return s.targetGroupARN
}

type stackOutput map[string]string

func newStackOutput(outputs []*cloudformation.Output) stackOutput {
	result := make(stackOutput)
	for _, o := range outputs {
		result[aws.StringValue(o.OutputKey)] = aws.StringValue(o.OutputValue)
	}
	return result
}

func (o stackOutput) dnsName() string {
	return o[outputLoadBalancerDNSName]
}

func (o stackOutput) targetGroupARN() string {
	return o[outputTargetGroupARN]
}

const (
	// The following constants should be part of the Output section of the CloudFormation template
	outputLoadBalancerDNSName = "LoadBalancerDNSName"
	outputTargetGroupARN      = "TargetGroupARN"

	parameterLoadBalancerSchemeParameter             = "LoadBalancerSchemeParameter"
	parameterLoadBalancerSecurityGroupParameter      = "LoadBalancerSecurityGroupParameter"
	parameterLoadBalancerSubnetsParameter            = "LoadBalancerSubnetsParameter"
	parameterTargetGroupHealthCheckPathParameter     = "TargetGroupHealthCheckPathParameter"
	parameterTargetGroupHealthCheckPortParameter     = "TargetGroupHealthCheckPortParameter"
	parameterTargetGroupHealthCheckIntervalParameter = "TargetGroupHealthCheckIntervalParameter"
	parameterTargetGroupVPCIDParameter               = "TargetGroupVPCIDParameter"
	parameterListenerCertificateParameter            = "ListenerCertificateParameter"
)

type createStackSpec struct {
	name             string
	scheme           string
	subnets          []string
	certificateARN   string
	securityGroupID  string
	clusterID        string
	vpcID            string
	healthCheck      *healthCheck
	timeoutInMinutes uint
	customTemplate   string
}

type healthCheck struct {
	path     string
	port     uint
	interval time.Duration
}

func createStack(svc cloudformationiface.CloudFormationAPI, spec *createStackSpec) (string, error) {
	template := templateYAML
	if spec.customTemplate != "" {
		template = spec.customTemplate
	}
	params := &cloudformation.CreateStackInput{
		StackName: aws.String(spec.name),
		OnFailure: aws.String(cloudformation.OnFailureDelete),
		Parameters: []*cloudformation.Parameter{
			cfParam(parameterLoadBalancerSchemeParameter, spec.scheme),
			cfParam(parameterLoadBalancerSecurityGroupParameter, spec.securityGroupID),
			cfParam(parameterLoadBalancerSubnetsParameter, strings.Join(spec.subnets, ",")),
			cfParam(parameterTargetGroupVPCIDParameter, spec.vpcID),
			cfParam(parameterListenerCertificateParameter, spec.certificateARN),
		},
		Tags: []*cloudformation.Tag{
			cfTag(kubernetesCreatorTag, kubernetesCreatorValue),
			cfTag(clusterIDTag, spec.clusterID),
		},
		TemplateBody:     aws.String(template),
		TimeoutInMinutes: aws.Int64(int64(spec.timeoutInMinutes)),
	}
	if spec.certificateARN != "" {
		params.Tags = append(params.Tags, cfTag(certificateARNTag, spec.certificateARN))
	}
	if spec.healthCheck != nil {
		params.Parameters = append(params.Parameters,
			cfParam(parameterTargetGroupHealthCheckPathParameter, spec.healthCheck.path),
			cfParam(parameterTargetGroupHealthCheckPortParameter, fmt.Sprintf("%d", spec.healthCheck.port)),
			cfParam(parameterTargetGroupHealthCheckIntervalParameter, fmt.Sprintf("%.0f", spec.healthCheck.interval.Seconds())),
		)
	}
	resp, err := svc.CreateStack(params)
	if err != nil {
		return spec.name, err
	}

	return aws.StringValue(resp.StackId), nil
}

func cfParam(key, value string) *cloudformation.Parameter {
	return &cloudformation.Parameter{
		ParameterKey:   aws.String(key),
		ParameterValue: aws.String(value),
	}
}

func cfTag(key, value string) *cloudformation.Tag {
	return &cloudformation.Tag{
		Key:   aws.String(key),
		Value: aws.String(value),
	}
}

func deleteStack(svc cloudformationiface.CloudFormationAPI, stackName string) error {
	params := &cloudformation.DeleteStackInput{StackName: aws.String(stackName)}
	if _, err := svc.DeleteStack(params); err != nil {
		return err
	}
	return nil
}

func getStack(svc cloudformationiface.CloudFormationAPI, stackName string) (*Stack, error) {
	params := &cloudformation.DescribeStacksInput{StackName: aws.String(stackName)}

	resp, err := svc.DescribeStacks(params)
	if err != nil {
		return nil, err
	}

	if len(resp.Stacks) < 1 {
		return nil, ErrLoadBalancerStackNotFound
	}

	var stack *cloudformation.Stack
	for _, s := range resp.Stacks {
		if isComplete(s.StackStatus) {
			stack = s
			break
		}
	}
	if stack == nil {
		return nil, ErrLoadBalancerStackNotReady
	}

	return mapToManagedStack(stack), nil
}

func mapToManagedStack(stack *cloudformation.Stack) *Stack {
	o, t := newStackOutput(stack.Outputs), convertCloudFormationTags(stack.Tags)
	return &Stack{
		name:           aws.StringValue(stack.StackName),
		dnsName:        o.dnsName(),
		targetGroupARN: o.targetGroupARN(),
		certificateARN: t[certificateARNTag],
	}
}

func isComplete(stackStatus *string) bool {
	switch aws.StringValue(stackStatus) {
	case cloudformation.StackStatusCreateComplete:
		fallthrough
	case cloudformation.ResourceStatusUpdateComplete:
		return true
	}
	return false
}

func findManagedStacks(svc cloudformationiface.CloudFormationAPI, clusterID string) ([]*Stack, error) {
	stacks := make([]*Stack, 0)
	err := svc.DescribeStacksPages(&cloudformation.DescribeStacksInput{},
		func(page *cloudformation.DescribeStacksOutput, lastPage bool) bool {
			for _, s := range page.Stacks {
				if !isComplete(s.StackStatus) {
					continue
				}

				if isManagedStack(s.Tags, clusterID) {
					stacks = append(stacks, mapToManagedStack(s))
				}
			}
			return true
		})
	if err != nil {
		return nil, fmt.Errorf("findManagedStacks failed to list stacks: %v", err)
	}
	return stacks, nil
}

func isManagedStack(cfTags []*cloudformation.Tag, clusterID string) bool {
	tags := convertCloudFormationTags(cfTags)
	if tags[kubernetesCreatorTag] != kubernetesCreatorValue {
		return false
	}
	if tags[clusterIDTag] != clusterID {
		return false
	}
	return true
}

func convertCloudFormationTags(tags []*cloudformation.Tag) map[string]string {
	ret := make(map[string]string)
	for _, tag := range tags {
		ret[aws.StringValue(tag.Key)] = aws.StringValue(tag.Value)
	}
	return ret
}
