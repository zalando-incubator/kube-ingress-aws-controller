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
	cwAlarmConfigHashTag    = "cloudwatch:alarm-config-hash"
)

// Stack is a simple wrapper around a CloudFormation Stack.
type Stack struct {
	Name              string
	status            string
	statusReason      string
	DNSName           string
	Scheme            string
	SecurityGroup     string
	SSLPolicy         string
	IpAddressType     string
	LoadBalancerType  string
	HTTP2             bool
	OwnerIngress      string
	CWAlarmConfigHash string
	TargetGroupARNs   []string
	WAFWebACLID       string
	CertificateARNs   map[string]time.Time
	tags              map[string]string
	Stickiness        bool
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

// Err returns nil or an error describing the stack state.
func (s *Stack) Err() error {
	if s == nil {
		return nil
	}

	switch s.status {
	case cloudformation.StackStatusCreateInProgress,
		cloudformation.StackStatusCreateComplete,
		cloudformation.StackStatusUpdateInProgress,
		cloudformation.StackStatusUpdateComplete,
		cloudformation.StackStatusUpdateCompleteCleanupInProgress,
		cloudformation.StackStatusDeleteInProgress,
		cloudformation.StackStatusDeleteComplete:
		return nil
	}

	if s.statusReason != "" {
		return fmt.Errorf("unexpected status %s: %s", s.status, s.statusReason)
	}
	return fmt.Errorf("unexpected status %s", s.status)
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

func (o stackOutput) targetGroupARNs() (arns []string) {
	if arn, ok := o[outputTargetGroupARN]; ok {
		arns = append(arns, arn)
	}
	if arn, ok := o[outputHTTPTargetGroupARN]; ok {
		arns = append(arns, arn)
	}
	return
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
	outputHTTPTargetGroupARN  = "HTTPTargetGroupARN"

	parameterLoadBalancerSchemeParameter             = "LoadBalancerSchemeParameter"
	parameterLoadBalancerSecurityGroupParameter      = "LoadBalancerSecurityGroupParameter"
	parameterLoadBalancerSubnetsParameter            = "LoadBalancerSubnetsParameter"
	parameterTargetGroupHealthCheckPathParameter     = "TargetGroupHealthCheckPathParameter"
	parameterTargetGroupHealthCheckPortParameter     = "TargetGroupHealthCheckPortParameter"
	parameterTargetGroupHealthCheckIntervalParameter = "TargetGroupHealthCheckIntervalParameter"
	parameterTargetGroupHealthCheckTimeoutParameter  = "TargetGroupHealthCheckTimeoutParameter"
	parameterTargetGroupTargetPortParameter          = "TargetGroupTargetPortParameter"
	parameterTargetGroupHTTPTargetPortParameter      = "TargetGroupHTTPTargetPortParameter"
	parameterTargetGroupVPCIDParameter               = "TargetGroupVPCIDParameter"
	parameterListenerSslPolicyParameter              = "ListenerSslPolicyParameter"
	parameterIpAddressTypeParameter                  = "IpAddressType"
	parameterLoadBalancerTypeParameter               = "Type"
	parameterLoadBalancerWAFWebACLIDParameter        = "LoadBalancerWAFWebACLIDParameter"
	parameterHTTP2Parameter                          = "HTTP2"
	parameterStickinessParameter                     = "Stickiness"
)

type stackSpec struct {
	name                              string
	scheme                            string
	ownerIngress                      string
	subnets                           []string
	certificateARNs                   map[string]time.Time
	securityGroupID                   string
	clusterID                         string
	vpcID                             string
	healthCheck                       *healthCheck
	albHealthyThresholdCount          uint
	albUnhealthyThresholdCount        uint
	nlbHealthyThresholdCount          uint
	targetType                        string
	targetPort                        uint
	targetHTTPS                       bool
	httpDisabled                      bool
	httpTargetPort                    uint
	timeoutInMinutes                  uint
	stackTerminationProtection        bool
	idleConnectionTimeoutSeconds      uint
	deregistrationDelayTimeoutSeconds uint
	controllerID                      string
	sslPolicy                         string
	ipAddressType                     string
	loadbalancerType                  string
	albLogsS3Bucket                   string
	albLogsS3Prefix                   string
	wafWebAclId                       string
	nlbZoneAffinity                   string
	cwAlarms                          CloudWatchAlarmList
	httpRedirectToHTTPS               bool
	nlbCrossZone                      bool
	http2                             bool
	denyInternalDomains               bool
	denyInternalDomainsResponse       denyResp
	internalDomains                   []string
	tags                              map[string]string
	stickiness                        bool
}

type healthCheck struct {
	path     string
	port     uint
	interval time.Duration
	timeout  time.Duration
}

type denyResp struct {
	statusCode  int
	contentType string
	body        string
}

func createStack(svc cloudformationiface.CloudFormationAPI, spec *stackSpec) (string, error) {
	template, err := generateTemplate(spec)
	if err != nil {
		return "", err
	}

	stackTags := map[string]string{
		kubernetesCreatorTag:                spec.controllerID,
		clusterIDTagPrefix + spec.clusterID: resourceLifecycleOwned,
	}

	tags := mergeTags(spec.tags, stackTags)

	params := &cloudformation.CreateStackInput{
		StackName: aws.String(spec.name),
		OnFailure: aws.String(cloudformation.OnFailureDelete),
		Parameters: []*cloudformation.Parameter{
			cfParam(parameterLoadBalancerSchemeParameter, spec.scheme),
			cfParam(parameterLoadBalancerSecurityGroupParameter, spec.securityGroupID),
			cfParam(parameterLoadBalancerSubnetsParameter, strings.Join(spec.subnets, ",")),
			cfParam(parameterTargetGroupVPCIDParameter, spec.vpcID),
			cfParam(parameterTargetGroupTargetPortParameter, fmt.Sprintf("%d", spec.targetPort)),
			cfParam(parameterListenerSslPolicyParameter, spec.sslPolicy),
			cfParam(parameterIpAddressTypeParameter, spec.ipAddressType),
			cfParam(parameterLoadBalancerTypeParameter, spec.loadbalancerType),
			cfParam(parameterHTTP2Parameter, fmt.Sprintf("%t", spec.http2)),
			cfParam(parameterStickinessParameter, spec.stickiness),
		},
		Tags:                        tagMapToCloudformationTags(tags),
		TemplateBody:                aws.String(template),
		TimeoutInMinutes:            aws.Int64(int64(spec.timeoutInMinutes)),
		EnableTerminationProtection: aws.Bool(spec.stackTerminationProtection),
	}

	if spec.wafWebAclId != "" {
		params.Parameters = append(
			params.Parameters,
			cfParam(parameterLoadBalancerWAFWebACLIDParameter, spec.wafWebAclId),
		)
	}

	if !spec.httpDisabled && spec.httpTargetPort != spec.targetPort {
		params.Parameters = append(
			params.Parameters,
			cfParam(parameterTargetGroupHTTPTargetPortParameter, fmt.Sprintf("%d", spec.httpTargetPort)),
		)
	}

	for certARN, ttl := range spec.certificateARNs {
		params.Tags = append(params.Tags, cfTag(certificateARNTagPrefix+certARN, ttl.Format(time.RFC3339)))
	}

	if spec.healthCheck != nil {
		params.Parameters = append(params.Parameters,
			cfParam(parameterTargetGroupHealthCheckPathParameter, spec.healthCheck.path),
			cfParam(parameterTargetGroupHealthCheckPortParameter, fmt.Sprintf("%d", spec.healthCheck.port)),
			cfParam(parameterTargetGroupHealthCheckIntervalParameter, fmt.Sprintf("%.0f", spec.healthCheck.interval.Seconds())),
			cfParam(parameterTargetGroupHealthCheckTimeoutParameter, fmt.Sprintf("%.0f", spec.healthCheck.timeout.Seconds())),
		)
	}

	if spec.ownerIngress != "" {
		params.Tags = append(params.Tags, cfTag(ingressOwnerTag, spec.ownerIngress))
	}

	if len(spec.cwAlarms) > 0 {
		params.Tags = append(params.Tags, cfTag(cwAlarmConfigHashTag, spec.cwAlarms.Hash()))
	}

	resp, err := svc.CreateStack(params)
	if err != nil {
		return spec.name, err
	}

	return aws.StringValue(resp.StackId), nil
}

func updateStack(svc cloudformationiface.CloudFormationAPI, spec *stackSpec) (string, error) {
	template, err := generateTemplate(spec)
	if err != nil {
		return "", err
	}

	stackTags := map[string]string{
		kubernetesCreatorTag:                spec.controllerID,
		clusterIDTagPrefix + spec.clusterID: resourceLifecycleOwned,
	}

	tags := mergeTags(spec.tags, stackTags)

	params := &cloudformation.UpdateStackInput{
		StackName: aws.String(spec.name),
		Parameters: []*cloudformation.Parameter{
			cfParam(parameterLoadBalancerSchemeParameter, spec.scheme),
			cfParam(parameterLoadBalancerSecurityGroupParameter, spec.securityGroupID),
			cfParam(parameterLoadBalancerSubnetsParameter, strings.Join(spec.subnets, ",")),
			cfParam(parameterTargetGroupVPCIDParameter, spec.vpcID),
			cfParam(parameterTargetGroupTargetPortParameter, fmt.Sprintf("%d", spec.targetPort)),
			cfParam(parameterListenerSslPolicyParameter, spec.sslPolicy),
			cfParam(parameterIpAddressTypeParameter, spec.ipAddressType),
			cfParam(parameterLoadBalancerTypeParameter, spec.loadbalancerType),
			cfParam(parameterHTTP2Parameter, fmt.Sprintf("%t", spec.http2)),
			cfParam(parameterStickinessParameter, spec.stickiness),
		},
		Tags:         tagMapToCloudformationTags(tags),
		TemplateBody: aws.String(template),
	}

	if spec.wafWebAclId != "" {
		params.Parameters = append(
			params.Parameters,
			cfParam(parameterLoadBalancerWAFWebACLIDParameter, spec.wafWebAclId),
		)
	}

	if !spec.httpDisabled && spec.httpTargetPort != spec.targetPort {
		params.Parameters = append(
			params.Parameters,
			cfParam(parameterTargetGroupHTTPTargetPortParameter, fmt.Sprintf("%d", spec.httpTargetPort)),
		)
	}

	for certARN, ttl := range spec.certificateARNs {
		params.Tags = append(params.Tags, cfTag(certificateARNTagPrefix+certARN, ttl.Format(time.RFC3339)))
	}

	if spec.healthCheck != nil {
		params.Parameters = append(params.Parameters,
			cfParam(parameterTargetGroupHealthCheckPathParameter, spec.healthCheck.path),
			cfParam(parameterTargetGroupHealthCheckPortParameter, fmt.Sprintf("%d", spec.healthCheck.port)),
			cfParam(parameterTargetGroupHealthCheckIntervalParameter, fmt.Sprintf("%.0f", spec.healthCheck.interval.Seconds())),
			cfParam(parameterTargetGroupHealthCheckTimeoutParameter, fmt.Sprintf("%.0f", spec.healthCheck.timeout.Seconds())),
		)
	}

	if spec.ownerIngress != "" {
		params.Tags = append(params.Tags, cfTag(ingressOwnerTag, spec.ownerIngress))
	}

	if len(spec.cwAlarms) > 0 {
		params.Tags = append(params.Tags, cfTag(cwAlarmConfigHashTag, spec.cwAlarms.Hash()))
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

func mergeTags(tags ...map[string]string) map[string]string {
	mergedTags := make(map[string]string)
	for _, tagMap := range tags {
		for k, v := range tagMap {
			mergedTags[k] = v
		}
	}
	return mergedTags
}

func tagMapToCloudformationTags(tags map[string]string) []*cloudformation.Tag {
	cfTags := make([]*cloudformation.Tag, 0, len(tags))
	for k, v := range tags {
		tag := &cloudformation.Tag{
			Key:   aws.String(k),
			Value: aws.String(v),
		}
		cfTags = append(cfTags, tag)
	}
	return cfTags
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

	http2 := true
	if parameters[parameterHTTP2Parameter] == "false" {
		http2 = false
	}

	stickiness := false
	if parameters[parameterStickinessParameter] == "true" {
		stickiness = true
	}

	return &Stack{
		Name:              aws.StringValue(stack.StackName),
		DNSName:           outputs.dnsName(),
		TargetGroupARNs:   outputs.targetGroupARNs(),
		Scheme:            parameters[parameterLoadBalancerSchemeParameter],
		SecurityGroup:     parameters[parameterLoadBalancerSecurityGroupParameter],
		SSLPolicy:         parameters[parameterListenerSslPolicyParameter],
		IpAddressType:     parameters[parameterIpAddressTypeParameter],
		LoadBalancerType:  parameters[parameterLoadBalancerTypeParameter],
		HTTP2:             http2,
		CertificateARNs:   certificateARNs,
		tags:              tags,
		OwnerIngress:      ownerIngress,
		status:            aws.StringValue(stack.StackStatus),
		statusReason:      aws.StringValue(stack.StackStatusReason),
		CWAlarmConfigHash: tags[cwAlarmConfigHashTag],
		WAFWebACLID:       parameters[parameterLoadBalancerWAFWebACLIDParameter],
		Stickiness:        stickiness,
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
		return nil, fmt.Errorf("findManagedStacks failed to list stacks: %w", err)
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
