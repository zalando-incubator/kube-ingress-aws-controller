package aws

//go:generate go run gencftemplate.go

import (
	"fmt"
	"log"
	"strings"

	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
	yaml "gopkg.in/yaml.v2"
)

const (
	deleteScheduled = "deleteScheduled"
)

// Stack is a simple wrapper around a CloudFormation Stack.
type Stack struct {
	name            string
	dnsName         string
	scheme          string
	targetGroupARN  string
	CertificateARNs []string
	tags            map[string]string
}

func (s *Stack) Name() string {
	return s.name
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

// IsDeleteInProgress returns true if the stack has already a tag
// deleteScheduled.
func (s *Stack) IsDeleteInProgress() bool {
	if s == nil {
		return false
	}
	_, ok := s.tags[deleteScheduled]
	return ok
}

// ShouldDelete returns true if stack is marked to delete and the
// deleteScheduled tag is after time.Now(). In all other cases it
// returns false.
func (s *Stack) ShouldDelete() bool {
	if s == nil {
		return false
	}
	t0 := s.deleteTime()
	if t0 == nil {
		return false
	}
	now := time.Now()
	return now.After(*t0)
}

func (s *Stack) deleteTime() *time.Time {
	if s == nil {
		return nil
	}
	ts, ok := s.tags[deleteScheduled]
	if !ok {
		return nil
	}
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		log.Printf("Failed to parse time: %v", err)
		return nil
	}
	return &t
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
	certificateARNs  []string
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

type Certificate struct {
	CertificateArn string `yaml:"CertificateArn"`
}

// cfTemplate is an opauqe structure for unmarshaling a yaml cloudformation
// stack into in order to replace the list of certificates for the
// HTTPSListener.
type cfTemplate struct {
	AWSTemplateFormatVersion string      `yaml:"AWSTemplateFormatVersion"`
	Description              string      `yaml:"Description"`
	Parameters               interface{} `yaml:"Parameters"`
	Conditions               interface{} `yaml:"Conditions"`
	Resources                struct {
		HTTPListener  interface{} `yaml:"HTTPListener"`
		HTTPSListener struct {
			Type       string `yaml:"Type"`
			Condition  string `yaml:"Condition"`
			Properties struct {
				DefaultActions []struct {
					Type           string `yaml:"Type"`
					TargetGroupArn string `yaml:"TargetGroupArn"`
				} `yaml:"DefaultActions"`
				LoadBalancerArn string        `yaml:"LoadBalancerArn"`
				Port            int           `yaml:"Port"`
				Protocol        string        `yaml:"Protocol"`
				Certificates    []Certificate `yaml:"Certificates"`
			}
		}
		LB interface{} `yaml:"LB"`
		TG interface{} `yaml:"TG"`
	} `yaml:"Resources"`
	Outputs interface{} `yaml:"Outputs"`
}

// injectCertificates injects a list of certificates into the cloudformation
// template.
func injectCertificates(cfTmpl string, certs []string) (string, error) {
	var template cfTemplate
	err := yaml.Unmarshal([]byte(cfTmpl), &template)
	if err != nil {
		return "", err
	}

	certificates := make([]Certificate, len(certs))
	for i, cert := range certs {
		certificates[i] = Certificate{CertificateArn: cert}
	}

	template.Resources.HTTPSListener.Properties.Certificates = certificates

	data, err := yaml.Marshal(&template)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func createStack(svc cloudformationiface.CloudFormationAPI, spec *stackSpec) (string, error) {
	template := templateYAML
	if spec.customTemplate != "" {
		template = spec.customTemplate
	}

	template, err := injectCertificates(template, spec.certificateARNs)
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
			cfParam(parameterListenerCertificatesParameter, strings.Join(spec.certificateARNs, ",")),
		},
		Tags: []*cloudformation.Tag{
			cfTag(kubernetesCreatorTag, kubernetesCreatorValue),
			cfTag(clusterIDTagPrefix+spec.clusterID, resourceLifecycleOwned),
		},
		TemplateBody:     aws.String(template),
		TimeoutInMinutes: aws.Int64(int64(spec.timeoutInMinutes)),
	}
	if len(spec.certificateARNs) > 0 {
		params.Tags = append(params.Tags, cfTag(certificateARNsTag, strings.Join(spec.certificateARNs, ",")))
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
	template := templateYAML
	if spec.customTemplate != "" {
		template = spec.customTemplate
	}

	template, err := injectCertificates(template, spec.certificateARNs)
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
			cfParam(parameterListenerCertificatesParameter, strings.Join(spec.certificateARNs, ",")),
		},
		Tags: []*cloudformation.Tag{
			cfTag(kubernetesCreatorTag, kubernetesCreatorValue),
			cfTag(clusterIDTagPrefix+spec.clusterID, resourceLifecycleOwned),
		},
		TemplateBody: aws.String(template),
	}
	if len(spec.certificateARNs) > 0 {
		params.Tags = append(params.Tags, cfTag(certificateARNsTag, strings.Join(spec.certificateARNs, ",")))
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

// maybe use https://docs.aws.amazon.com/AWSCloudFormation/latest/APIReference/API_CreateChangeSet.html instead
func markToDeleteStack(svc cloudformationiface.CloudFormationAPI, stackName, ts string) error {
	stack, err := getCFStackByName(svc, stackName)
	if err != nil {
		return err
	}
	tags := append(stack.Tags, cfTag(deleteScheduled, ts))

	params := &cloudformation.UpdateStackInput{
		StackName:           aws.String(stackName),
		Tags:                tags,
		Parameters:          stack.Parameters,
		UsePreviousTemplate: aws.Bool(true),
	}

	_, err = svc.UpdateStack(params)
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
		if isComplete(s.StackStatus) {
			stack = s
			break
		}
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

	return &Stack{
		name:            aws.StringValue(stack.StackName),
		dnsName:         outputs.dnsName(),
		targetGroupARN:  outputs.targetGroupARN(),
		scheme:          parameters[parameterLoadBalancerSchemeParameter],
		CertificateARNs: strings.Split(tags[certificateARNsTag], ","),
		tags:            tags,
	}
}

// isComplete returns false by design on all other status, because
// updateIngress will ignore not completed stacks.
// Stack can never be in rollback state by design.
func isComplete(stackStatus *string) bool {
	switch aws.StringValue(stackStatus) {
	case cloudformation.StackStatusCreateComplete:
		return true
	case cloudformation.StackStatusUpdateComplete:
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
