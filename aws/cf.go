package aws

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	elbv2Types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
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
	status            types.StackStatus
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
}

// IsComplete returns true if the stack status is a complete state.
func (s *Stack) IsComplete() bool {
	if s == nil {
		return false
	}

	switch types.StackStatus(s.status) {
	case types.StackStatusCreateComplete,
		types.StackStatusUpdateComplete,
		types.StackStatusRollbackComplete,
		types.StackStatusUpdateRollbackComplete:
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
	case types.StackStatusCreateInProgress,
		types.StackStatusCreateComplete,
		types.StackStatusUpdateInProgress,
		types.StackStatusUpdateComplete,
		types.StackStatusUpdateCompleteCleanupInProgress,
		types.StackStatusDeleteInProgress,
		types.StackStatusDeleteComplete:
		return nil
	}

	if s.statusReason != "" {
		return fmt.Errorf("unexpected status %s: %s", s.status, s.statusReason)
	}
	return fmt.Errorf("unexpected status %s", s.status)
}

type stackOutput map[string]string

func newStackOutput(outputs []types.Output) stackOutput {
	result := make(stackOutput)
	for _, o := range outputs {
		result[aws.ToString(o.OutputKey)] = aws.ToString(o.OutputValue)
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
func convertStackParameters(parameters []types.Parameter) map[string]string {
	result := make(map[string]string)
	for _, p := range parameters {
		result[aws.ToString(p.ParameterKey)] = aws.ToString(p.ParameterValue)
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
	targetType                        elbv2Types.TargetTypeEnum
	targetPort                        uint
	targetHTTPS                       bool
	httpDisabled                      bool
	httpTargetPort                    uint
	timeoutInMinutes                  int32
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

type CloudFormationAPI interface {
	cloudformation.DescribeStacksAPIClient
	cloudformation.ListStackResourcesAPIClient
	CreateStack(context.Context, *cloudformation.CreateStackInput, ...func(*cloudformation.Options)) (*cloudformation.CreateStackOutput, error)
	UpdateTerminationProtection(context.Context, *cloudformation.UpdateTerminationProtectionInput, ...func(*cloudformation.Options)) (*cloudformation.UpdateTerminationProtectionOutput, error)
	UpdateStack(context.Context, *cloudformation.UpdateStackInput, ...func(*cloudformation.Options)) (*cloudformation.UpdateStackOutput, error)
	DeleteStack(context.Context, *cloudformation.DeleteStackInput, ...func(*cloudformation.Options)) (*cloudformation.DeleteStackOutput, error)
}

func createStack(ctx context.Context, svc CloudFormationAPI, spec *stackSpec) (string, error) {
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
		OnFailure: types.OnFailureDelete,
		Parameters: []types.Parameter{
			cfParam(parameterLoadBalancerSchemeParameter, spec.scheme),
			cfParam(parameterLoadBalancerSecurityGroupParameter, spec.securityGroupID),
			cfParam(parameterLoadBalancerSubnetsParameter, strings.Join(spec.subnets, ",")),
			cfParam(parameterTargetGroupVPCIDParameter, spec.vpcID),
			cfParam(parameterTargetGroupTargetPortParameter, fmt.Sprintf("%d", spec.targetPort)),
			cfParam(parameterListenerSslPolicyParameter, spec.sslPolicy),
			cfParam(parameterIpAddressTypeParameter, spec.ipAddressType),
			cfParam(parameterLoadBalancerTypeParameter, spec.loadbalancerType),
			cfParam(parameterHTTP2Parameter, fmt.Sprintf("%t", spec.http2)),
		},
		Tags:                        tagMapToCloudformationTags(tags),
		TemplateBody:                aws.String(template),
		TimeoutInMinutes:            aws.Int32(int32(spec.timeoutInMinutes)),
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

	resp, err := svc.CreateStack(ctx, params)
	if err != nil {
		return spec.name, err
	}

	return aws.ToString(resp.StackId), nil
}

func updateStack(ctx context.Context, svc CloudFormationAPI, spec *stackSpec) (string, error) {
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
		Parameters: []types.Parameter{
			cfParam(parameterLoadBalancerSchemeParameter, spec.scheme),
			cfParam(parameterLoadBalancerSecurityGroupParameter, spec.securityGroupID),
			cfParam(parameterLoadBalancerSubnetsParameter, strings.Join(spec.subnets, ",")),
			cfParam(parameterTargetGroupVPCIDParameter, spec.vpcID),
			cfParam(parameterTargetGroupTargetPortParameter, fmt.Sprintf("%d", spec.targetPort)),
			cfParam(parameterListenerSslPolicyParameter, spec.sslPolicy),
			cfParam(parameterIpAddressTypeParameter, spec.ipAddressType),
			cfParam(parameterLoadBalancerTypeParameter, spec.loadbalancerType),
			cfParam(parameterHTTP2Parameter, fmt.Sprintf("%t", spec.http2)),
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

		_, err := svc.UpdateTerminationProtection(ctx, params)
		if err != nil {
			return spec.name, err
		}
	}

	resp, err := svc.UpdateStack(ctx, params)
	if err != nil {
		return spec.name, err
	}

	return aws.ToString(resp.StackId), nil
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

func tagMapToCloudformationTags(tags map[string]string) []types.Tag {
	cfTags := make([]types.Tag, 0, len(tags))
	for k, v := range tags {
		tag := types.Tag{
			Key:   aws.String(k),
			Value: aws.String(v),
		}
		cfTags = append(cfTags, tag)
	}
	return cfTags
}

func cfParam(key, value string) types.Parameter {
	return types.Parameter{
		ParameterKey:   aws.String(key),
		ParameterValue: aws.String(value),
	}
}

func cfTag(key, value string) types.Tag {
	return types.Tag{
		Key:   aws.String(key),
		Value: aws.String(value),
	}
}

func deleteStack(ctx context.Context, svc CloudFormationAPI, stackName string) error {
	termParams := &cloudformation.UpdateTerminationProtectionInput{
		StackName:                   aws.String(stackName),
		EnableTerminationProtection: aws.Bool(false),
	}

	_, err := svc.UpdateTerminationProtection(ctx, termParams)
	if err != nil {
		return err
	}

	params := &cloudformation.DeleteStackInput{StackName: aws.String(stackName)}
	_, err = svc.DeleteStack(ctx, params)
	return err
}

func getStack(ctx context.Context, svc CloudFormationAPI, stackName string) (*Stack, error) {
	stack, err := getCFStackByName(ctx, svc, stackName)
	if err != nil {
		return nil, ErrLoadBalancerStackNotReady
	}
	return mapToManagedStack(stack), nil
}

// getLoadBalancerStackResource retrieves the load balancer resource from a
// CloudFormation stack. It returns the first resource of type
// AWS::ElasticLoadBalancingV2::LoadBalancer found in the stack. The stack
// should only have one such resource, as it is expected to be a managed load
// balancer stack.
func getLoadBalancerStackResource(
	ctx context.Context,
	svc CloudFormationAPI,
	stackName string,
) (
	*types.StackResourceSummary,
	error,
) {

	var nextToken *string

	for {
		resp, err := svc.ListStackResources(ctx, &cloudformation.ListStackResourcesInput{
			StackName: aws.String(stackName),
			NextToken: nextToken,
		})

		if err != nil {
			return nil, fmt.Errorf("failed to list stack resources for stack %s: %w", stackName, err)
		}

		for _, resource := range resp.StackResourceSummaries {
			if aws.ToString(resource.LogicalResourceId) == loadBalancerResourceLogicalID {
				return &resource, nil
			}
		}

		if resp.NextToken == nil {
			return nil, fmt.Errorf("no load balancer resource found in stack %s", stackName)
		}

		nextToken = resp.NextToken
	}
}

func getCFStackByName(ctx context.Context, svc CloudFormationAPI, stackName string) (*types.Stack, error) {
	params := &cloudformation.DescribeStacksInput{StackName: aws.String(stackName)}

	resp, err := svc.DescribeStacks(ctx, params)
	if err != nil {
		return nil, err
	}

	if len(resp.Stacks) < 1 {
		return nil, ErrLoadBalancerStackNotFound
	}

	// TODO: log/error on multiple stacks

	return &resp.Stacks[0], nil
}

func mapToManagedStack(stack *types.Stack) *Stack {
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

	return &Stack{
		Name:              aws.ToString(stack.StackName),
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
		status:            stack.StackStatus,
		statusReason:      aws.ToString(stack.StackStatusReason),
		CWAlarmConfigHash: tags[cwAlarmConfigHashTag],
		WAFWebACLID:       parameters[parameterLoadBalancerWAFWebACLIDParameter],
	}
}

func findManagedStacks(ctx context.Context, svc CloudFormationAPI, clusterID, controllerID string) ([]*Stack, error) {
	stacks := make([]*Stack, 0)
	paginator := cloudformation.NewDescribeStacksPaginator(svc, &cloudformation.DescribeStacksInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("findManagedStacks failed to list stacks: %w", err)
		}
		for _, s := range page.Stacks {
			if isManagedStack(s.Tags, clusterID, controllerID) {
				stacks = append(stacks, mapToManagedStack(&s))
			}
		}
	}
	return stacks, nil
}

func isManagedStack(cfTags []types.Tag, clusterID string, controllerID string) bool {
	tags := convertCloudFormationTags(cfTags)

	if tags[kubernetesCreatorTag] != controllerID {
		return false
	}

	// TODO(sszuecs): remove 2nd condition, only for migration
	return tags[clusterIDTagPrefix+clusterID] == resourceLifecycleOwned || tags[clusterIDTag] == clusterID
}

func convertCloudFormationTags(tags []types.Tag) map[string]string {
	ret := make(map[string]string)
	for _, tag := range tags {
		ret[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}
	return ret
}
