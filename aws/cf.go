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
	ingressOwnerTag         = "ingress:owner"
)

// Stack is a simple wrapper around a CloudFormation Stack.
type Stack struct {
	Name            string
	status          string
	DNSName         string
	Scheme          string
	TargetGroupARN  string
	CertificateARNs map[string]time.Time
	OwnerIngress    string
	tags            map[string]string
}

// IsComplete returns true if the stack status is a complete state.
func (s *Stack) IsComplete() bool {
	if s == nil {
		return false
	}

	switch s.status {
	case cloudformation.StackStatusCreateComplete,
		cloudformation.StackStatusUpdateComplete,
		cloudformation.StackStatusRollbackComplete,
		cloudformation.StackStatusUpdateRollbackComplete:
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
	for _, t := range s.CertificateARNs {
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
	parameterTargetTargetPortParameter               = "TargetGroupTargetPortParameter"
	parameterTargetGroupVPCIDParameter               = "TargetGroupVPCIDParameter"
	parameterListenerCertificatesParameter           = "ListenerCertificatesParameter"
	parameterListenerSslPolicyParameter              = "ListenerSslPolicyParameter"
	parameterIpAddressTypeParameter                  = "IpAddressType"
)

type stackSpec struct {
	name                         string
	scheme                       string
	ownerIngress                 string
	subnets                      []string
	certificateARNs              map[string]time.Time
	securityGroupID              string
	clusterID                    string
	vpcID                        string
	healthCheck                  *healthCheck
	targetPort                   uint
	timeoutInMinutes             uint
	customTemplate               string
	stackTerminationProtection   bool
	idleConnectionTimeoutSeconds uint
	controllerID                 string
	sslPolicy                    string
	ipAddressType                string
	albLogsS3Bucket              string
	albLogsS3Prefix              string
}

type healthCheck struct {
	path     string
	port     uint
	interval time.Duration
}

func createStack(svc cloudformationiface.CloudFormationAPI, spec *stackSpec) (string, error) {
	template, err := generateTemplate(spec.certificateARNs, spec.idleConnectionTimeoutSeconds, spec.albLogsS3Bucket, spec.albLogsS3Prefix)
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
			cfParam(parameterTargetTargetPortParameter, fmt.Sprintf("%d", spec.targetPort)),
			cfParam(parameterListenerSslPolicyParameter, spec.sslPolicy),
			cfParam(parameterIpAddressTypeParameter, spec.ipAddressType),
		},
		Tags: []*cloudformation.Tag{
			cfTag(kubernetesCreatorTag, spec.controllerID),
			cfTag(clusterIDTagPrefix+spec.clusterID, resourceLifecycleOwned),
		},
		TemplateBody:                aws.String(template),
		TimeoutInMinutes:            aws.Int64(int64(spec.timeoutInMinutes)),
		EnableTerminationProtection: aws.Bool(spec.stackTerminationProtection),
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

	if spec.ownerIngress != "" {
		params.Tags = append(params.Tags, cfTag(ingressOwnerTag, spec.ownerIngress))
	}

	resp, err := svc.CreateStack(params)
	if err != nil {
		return spec.name, err
	}

	return aws.StringValue(resp.StackId), nil
}

func updateStack(svc cloudformationiface.CloudFormationAPI, spec *stackSpec) (string, error) {
	template, err := generateTemplate(spec.certificateARNs, spec.idleConnectionTimeoutSeconds, spec.albLogsS3Bucket, spec.albLogsS3Prefix)
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
			cfParam(parameterTargetTargetPortParameter, fmt.Sprintf("%d", spec.targetPort)),
			cfParam(parameterListenerSslPolicyParameter, spec.sslPolicy),
			cfParam(parameterIpAddressTypeParameter, spec.ipAddressType),
		},
		Tags: []*cloudformation.Tag{
			cfTag(kubernetesCreatorTag, spec.controllerID),
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

	if spec.ownerIngress != "" {
		params.Tags = append(params.Tags, cfTag(ingressOwnerTag, spec.ownerIngress))
	}

	if spec.stackTerminationProtection {
		params := &cloudformation.UpdateTerminationProtectionInput{
			StackName:                   aws.String(spec.name),
			EnableTerminationProtection: aws.Bool(spec.stackTerminationProtection),
		}

		_, err := svc.UpdateTerminationProtection(params)
		if err != nil {
			return spec.name, err
		}
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
	termParams := &cloudformation.UpdateTerminationProtectionInput{
		StackName:                   aws.String(stackName),
		EnableTerminationProtection: aws.Bool(false),
	}

	_, err := svc.UpdateTerminationProtection(termParams)
	if err != nil {
		return err
	}

	params := &cloudformation.DeleteStackInput{StackName: aws.String(stackName)}
	_, err = svc.DeleteStack(params)
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
	ownerIngress := ""
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

		if key == ingressOwnerTag {
			ownerIngress = value
		}
	}

	return &Stack{
		Name:            aws.StringValue(stack.StackName),
		DNSName:         outputs.dnsName(),
		TargetGroupARN:  outputs.targetGroupARN(),
		Scheme:          parameters[parameterLoadBalancerSchemeParameter],
		CertificateARNs: certificateARNs,
		tags:            tags,
		OwnerIngress:    ownerIngress,
		status:          aws.StringValue(stack.StackStatus),
	}
}

func findManagedStacks(svc cloudformationiface.CloudFormationAPI, clusterID, controllerID string) ([]*Stack, error) {
	stacks := make([]*Stack, 0)
	err := svc.DescribeStacksPages(&cloudformation.DescribeStacksInput{},
		func(page *cloudformation.DescribeStacksOutput, lastPage bool) bool {
			for _, s := range page.Stacks {
				if isManagedStack(s.Tags, clusterID, controllerID) {
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

func isManagedStack(cfTags []*cloudformation.Tag, clusterID string, controllerID string) bool {
	tags := convertCloudFormationTags(cfTags)

	if tags[kubernetesCreatorTag] != controllerID {
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
