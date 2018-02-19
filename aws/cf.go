package aws

import (
	"fmt"
	"strings"

	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
)

const (
	certificateARNTagLegacy = "ingress:certificate-arn"
	certificateARNTagPrefix = "ingress:certificate-arn/"
)

// Stack is a simple wrapper around a CloudFormation Stack.
type Stack struct {
	name            string
	status          string
	dnsName         string
	scheme          string
	targetGroupARN  string
	certificateARNs map[string]time.Time
	tags            map[string]string
}

func (s *Stack) Name() string {
	return s.name
}

func (s *Stack) CertificateARNs() map[string]time.Time {
	return s.certificateARNs
}

func (s *Stack) DNSName() string {
	return s.dnsName
}

func (s *Stack) Scheme() string {
	return s.scheme
}

func (s *Stack) TargetGroupARN() string {
	return s.targetGroupARN
}

// IsComplete returns true if the stack status is a complete state.
func (s *Stack) IsComplete() bool {
	if s == nil {
		return false
	}

	switch s.status {
	case cloudformation.StackStatusCreateComplete:
		return true
	case cloudformation.StackStatusUpdateComplete:
		return true

	}
	return false
}

// ShouldDelete returns true if stack is to be deleted because there are no
// valid certificates attached anymore.
func (s *Stack) ShouldDelete() bool {
	if s == nil {
		return false
	}

	now := time.Now().UTC()
	for _, t := range s.certificateARNs {
		if t.IsZero() || t.After(now) {
			return false
		}
	}

	return true
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

// convertStackParameters converts a list of cloudformation stack parameters to
// a map.
func convertStackParameters(parameters []*cloudformation.Parameter) map[string]string {
	result := make(map[string]string)
	for _, p := range parameters {
		result[aws.StringValue(p.ParameterKey)] = aws.StringValue(p.ParameterValue)
	}
	return result
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
	parameterListenerCertificatesParameter           = "ListenerCertificatesParameter"
)

type stackSpec struct {
	name             string
	scheme           string
	subnets          []string
	certificateARNs  map[string]time.Time
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

func createStack(svc cloudformationiface.CloudFormationAPI, spec *stackSpec) (string, error) {
	template, err := generateTemplate(spec.certificateARNs)
	if err != nil {
		return "", err
	}

	params := &cloudformation.CreateStackInput{
		StackName: aws.String(spec.name),
		OnFailure: aws.String(cloudformation.OnFailureDelete),
		Parameters: []*cloudformation.Parameter{
			cfParam(parameterLoadBalancerSchemeParameter, spec.scheme),
			cfParam(parameterLoadBalancerSecurityGroupParameter, spec.securityGroupID),
			cfParam(parameterLoadBalancerSubnetsParameter, strings.Join(spec.subnets, ",")),
			cfParam(parameterTargetGroupVPCIDParameter, spec.vpcID),
		},
		Tags: []*cloudformation.Tag{
			cfTag(kubernetesCreatorTag, kubernetesCreatorValue),
			cfTag(clusterIDTagPrefix+spec.clusterID, resourceLifecycleOwned),
		},
		TemplateBody:     aws.String(template),
		TimeoutInMinutes: aws.Int64(int64(spec.timeoutInMinutes)),
	}

	for certARN, ttl := range spec.certificateARNs {
		params.Tags = append(params.Tags, cfTag(certificateARNTagPrefix+certARN, ttl.Format(time.RFC3339)))
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

func updateStack(svc cloudformationiface.CloudFormationAPI, spec *stackSpec) (string, error) {
	template, err := generateTemplate(spec.certificateARNs)
	if err != nil {
		return "", err
	}

	params := &cloudformation.UpdateStackInput{
		StackName: aws.String(spec.name),
		Parameters: []*cloudformation.Parameter{
			cfParam(parameterLoadBalancerSchemeParameter, spec.scheme),
			cfParam(parameterLoadBalancerSecurityGroupParameter, spec.securityGroupID),
			cfParam(parameterLoadBalancerSubnetsParameter, strings.Join(spec.subnets, ",")),
			cfParam(parameterTargetGroupVPCIDParameter, spec.vpcID),
		},
		Tags: []*cloudformation.Tag{
			cfTag(kubernetesCreatorTag, kubernetesCreatorValue),
			cfTag(clusterIDTagPrefix+spec.clusterID, resourceLifecycleOwned),
		},
		TemplateBody: aws.String(template),
	}

	for certARN, ttl := range spec.certificateARNs {
		params.Tags = append(params.Tags, cfTag(certificateARNTagPrefix+certARN, ttl.Format(time.RFC3339)))
	}

	if spec.healthCheck != nil {
		params.Parameters = append(params.Parameters,
			cfParam(parameterTargetGroupHealthCheckPathParameter, spec.healthCheck.path),
			cfParam(parameterTargetGroupHealthCheckPortParameter, fmt.Sprintf("%d", spec.healthCheck.port)),
			cfParam(parameterTargetGroupHealthCheckIntervalParameter, fmt.Sprintf("%.0f", spec.healthCheck.interval.Seconds())),
		)
	}
	resp, err := svc.UpdateStack(params)
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
	_, err := svc.DeleteStack(params)
	return err
}

func getStack(svc cloudformationiface.CloudFormationAPI, stackName string) (*Stack, error) {
	stack, err := getCFStackByName(svc, stackName)
	if err != nil {
		return nil, ErrLoadBalancerStackNotReady
	}
	return mapToManagedStack(stack), nil
}

func getCFStackByName(svc cloudformationiface.CloudFormationAPI, stackName string) (*cloudformation.Stack, error) {
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
		stack = s
		break
	}
	if stack == nil {
		return nil, ErrLoadBalancerStackNotReady
	}

	return stack, nil
}

func mapToManagedStack(stack *cloudformation.Stack) *Stack {
	outputs := newStackOutput(stack.Outputs)
	tags := convertCloudFormationTags(stack.Tags)
	parameters := convertStackParameters(stack.Parameters)

	certificateARNs := make(map[string]time.Time, len(tags))
	for key, value := range tags {
		if strings.HasPrefix(key, certificateARNTagPrefix) {
			arn := strings.TrimPrefix(key, certificateARNTagPrefix)
			ttl, err := time.Parse(time.RFC3339, value)
			if err != nil {
				ttl = time.Time{} // zero value
			}
			certificateARNs[arn] = ttl
		}

		// TODO(mlarsen): used for migrating from old format to new.
		// Should be removed in a later version.
		if key == certificateARNTagLegacy {
			certificateARNs[value] = time.Time{}
		}
	}

	return &Stack{
		name:            aws.StringValue(stack.StackName),
		dnsName:         outputs.dnsName(),
		targetGroupARN:  outputs.targetGroupARN(),
		scheme:          parameters[parameterLoadBalancerSchemeParameter],
		certificateARNs: certificateARNs,
		tags:            tags,
		status:          aws.StringValue(stack.StackStatus),
	}
}

func findManagedStacks(svc cloudformationiface.CloudFormationAPI, clusterID string) ([]*Stack, error) {
	stacks := make([]*Stack, 0)
	err := svc.DescribeStacksPages(&cloudformation.DescribeStacksInput{},
		func(page *cloudformation.DescribeStacksOutput, lastPage bool) bool {
			for _, s := range page.Stacks {
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
	// TODO(sszuecs): remove 2nd condition, only for migration
	return tags[clusterIDTagPrefix+clusterID] == resourceLifecycleOwned || tags[clusterIDTag] == clusterID
}

func convertCloudFormationTags(tags []*cloudformation.Tag) map[string]string {
	ret := make(map[string]string)
	for _, tag := range tags {
		ret[aws.StringValue(tag.Key)] = aws.StringValue(tag.Value)
	}
	return ret
}
