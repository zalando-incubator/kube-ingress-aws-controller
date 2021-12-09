package aws

import (
	"encoding/json"
	"fmt"
	"sort"
	"testing"
	"time"

	cloudformation "github.com/o11n/go-cloudformation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateTemplate(t *testing.T) {
	internalDomains := []string{"*.ingress.cluster.local", "*.internal.domain.local"}
	denyResp := denyResp{
		statusCode:  404,
		contentType: "text/plain",
		body:        "Not Found",
	}

	validateDenyRule := func(t *testing.T, resource *cloudformation.Resource) {
		rules, ok := resource.Properties.(*cloudformation.ElasticLoadBalancingV2ListenerRule)
		require.True(t, ok, "couldn't convert resource to ElasticLoadBalancingV2ListenerRule")

		conditions := *rules.Conditions
		require.Equal(t, len(conditions), 1)
		condition := conditions[0]

		actions := *rules.Actions
		require.Equal(t, len(actions), 1)
		action := actions[0]

		require.Equal(t, internalTrafficDenyRulePriority, rules.Priority.Literal)

		require.Equal(t, listenerRuleConditionHostField, condition.Field.Literal)
		var conditionValues []string
		for _, v := range condition.Values.Literal {
			conditionValues = append(conditionValues, v.Literal)
		}
		require.Equal(t, len(internalDomains), len(conditionValues))
		sort.Strings(conditionValues)
		require.Equal(t, internalDomains, conditionValues)

		require.Equal(t, listenerRuleActionTypeFixedRes, action.Type.Literal)
		require.Equal(t, denyResp.contentType, action.FixedResponseConfig.ContentType.Literal)
		require.Equal(t, denyResp.body, action.FixedResponseConfig.MessageBody.Literal)
		require.Equal(t, fmt.Sprintf("%d", denyResp.statusCode), action.FixedResponseConfig.StatusCode.Literal)
	}

	for _, test := range []struct {
		name     string
		spec     *stackSpec
		validate func(t *testing.T, template *cloudformation.Template)
	}{
		{
			name: "contains no cloudwatch alarm resources if spec has none",
			spec: &stackSpec{},
			validate: func(t *testing.T, template *cloudformation.Template) {
				require.Nil(t, template.Resources["CloudWatchAlarm0"])
				require.Nil(t, template.Resources["CloudWatchAlarm1"])
			},
		},
		{
			name: "contains cloudwatch alarm resources #1",
			spec: &stackSpec{
				cwAlarms: CloudWatchAlarmList{
					{
						AlarmName: cloudformation.String("foo"),
					},
				},
			},
			validate: func(t *testing.T, template *cloudformation.Template) {
				require.NotNil(t, template.Resources["CloudWatchAlarm0"])

				alarm := template.Resources["CloudWatchAlarm0"]

				props := alarm.Properties.(*cloudformation.CloudWatchAlarm)

				assert.Equal(
					t,
					cloudformation.Join(
						"-",
						cloudformation.Ref("AWS::StackName"),
						cloudformation.String("foo"),
					),
					props.AlarmName,
				)
				assert.Equal(
					t,
					cloudformation.String("AWS/ApplicationELB"),
					props.Namespace,
				)
			},
		},
		{
			name: "contains cloudwatch alarm resources #1",
			spec: &stackSpec{
				cwAlarms: CloudWatchAlarmList{
					{},
					{
						Namespace: cloudformation.String("AWS/Whatever"),
						Dimensions: &cloudformation.CloudWatchAlarmDimensionList{
							{
								Name:  cloudformation.String("LoadBalancer"),
								Value: cloudformation.String("foo-lb"),
							},
						},
					},
				},
			},
			validate: func(t *testing.T, template *cloudformation.Template) {
				require.NotNil(t, template.Resources["CloudWatchAlarm0"])
				require.NotNil(t, template.Resources["CloudWatchAlarm1"])

				alarm := template.Resources["CloudWatchAlarm1"]

				props := alarm.Properties.(*cloudformation.CloudWatchAlarm)

				assert.Equal(
					t,
					&cloudformation.CloudWatchAlarmDimensionList{
						{
							Name:  cloudformation.String("LoadBalancer"),
							Value: cloudformation.GetAtt("LB", "LoadBalancerFullName").String(),
						},
					},
					props.Dimensions,
				)
				assert.Equal(
					t,
					cloudformation.String("AWS/Whatever"),
					props.Namespace,
				)
			},
		},
		{
			name: "ALB should only have HTTP listener when no certificate is present",
			spec: &stackSpec{
				loadbalancerType: LoadBalancerTypeApplication,
			},
			validate: func(t *testing.T, template *cloudformation.Template) {
				requireTargetGroups(t, template, "TG")
				requireListeners(t, template, "HTTPListener")

				tg := template.Resources["TG"].Properties.(*cloudformation.ElasticLoadBalancingV2TargetGroup)

				require.Contains(t, template.Parameters, parameterTargetGroupTargetPortParameter)
				require.Equal(t, cloudformation.Ref(parameterTargetGroupTargetPortParameter).Integer(), tg.Port)
				require.Equal(t, cloudformation.String("HTTP"), tg.Protocol)
				require.Equal(t, cloudformation.String("HTTP"), tg.HealthCheckProtocol)

				validateTargetGroupListener(t, template, "TG", "HTTPListener", 80, "HTTP")
				validateTargetGroupOutput(t, template, "TG", "TargetGroupARN")
			},
		},
		{
			name: "ALB should have two listeners when certificate is present",
			spec: &stackSpec{
				loadbalancerType: LoadBalancerTypeApplication,
				certificateARNs:  map[string]time.Time{"domain.company.com": time.Now()},
			},
			validate: func(t *testing.T, template *cloudformation.Template) {
				requireTargetGroups(t, template, "TG")
				requireListeners(t, template, "HTTPListener", "HTTPSListener")

				tg := template.Resources["TG"].Properties.(*cloudformation.ElasticLoadBalancingV2TargetGroup)

				require.Contains(t, template.Parameters, parameterTargetGroupTargetPortParameter)
				require.Equal(t, cloudformation.Ref(parameterTargetGroupTargetPortParameter).Integer(), tg.Port)
				require.Equal(t, cloudformation.String("HTTP"), tg.Protocol)
				require.Equal(t, cloudformation.String("HTTP"), tg.HealthCheckProtocol)

				validateTargetGroupListener(t, template, "TG", "HTTPListener", 80, "HTTP")
				validateTargetGroupListener(t, template, "TG", "HTTPSListener", 443, "HTTPS")
				validateTargetGroupOutput(t, template, "TG", "TargetGroupARN")
			},
		},
		{
			name: "ALB should use separate Target Group for HTTP when HTTP target port is configured",
			spec: &stackSpec{
				loadbalancerType: LoadBalancerTypeApplication,
				certificateARNs:  map[string]time.Time{"domain.company.com": time.Now()},
				httpTargetPort:   8888,
			},
			validate: func(t *testing.T, template *cloudformation.Template) {
				requireTargetGroups(t, template, "TGHTTP", "TG")
				requireListeners(t, template, "HTTPListener", "HTTPSListener")

				tgHTTP := template.Resources["TGHTTP"].Properties.(*cloudformation.ElasticLoadBalancingV2TargetGroup)
				require.Contains(t, template.Parameters, parameterTargetGroupHTTPTargetPortParameter)
				require.Equal(t, cloudformation.Ref(parameterTargetGroupHTTPTargetPortParameter).Integer(), tgHTTP.Port)
				require.Equal(t, cloudformation.String("HTTP"), tgHTTP.Protocol)
				require.Equal(t, cloudformation.String("HTTP"), tgHTTP.HealthCheckProtocol)

				validateTargetGroupListener(t, template, "TGHTTP", "HTTPListener", 80, "HTTP")
				validateTargetGroupOutput(t, template, "TGHTTP", "HTTPTargetGroupARN")

				tg := template.Resources["TG"].Properties.(*cloudformation.ElasticLoadBalancingV2TargetGroup)
				require.Contains(t, template.Parameters, parameterTargetGroupTargetPortParameter)
				require.Equal(t, cloudformation.Ref(parameterTargetGroupTargetPortParameter).Integer(), tg.Port)
				require.Equal(t, cloudformation.String("HTTP"), tg.Protocol)
				require.Equal(t, cloudformation.String("HTTP"), tg.HealthCheckProtocol)

				validateTargetGroupListener(t, template, "TG", "HTTPSListener", 443, "HTTPS")
				validateTargetGroupOutput(t, template, "TG", "TargetGroupARN")
			},
		},
		{
			name: "ALB should use one Target Group when HTTP target port equals target port",
			spec: &stackSpec{
				loadbalancerType: LoadBalancerTypeApplication,
				certificateARNs:  map[string]time.Time{"domain.company.com": time.Now()},
				targetPort:       9999,
				httpTargetPort:   9999,
			},
			validate: func(t *testing.T, template *cloudformation.Template) {
				requireTargetGroups(t, template, "TG")
				requireListeners(t, template, "HTTPListener", "HTTPSListener")

				tg := template.Resources["TG"].Properties.(*cloudformation.ElasticLoadBalancingV2TargetGroup)
				require.NotContains(t, template.Parameters, parameterTargetGroupHTTPTargetPortParameter)
				require.Contains(t, template.Parameters, parameterTargetGroupTargetPortParameter)
				require.Equal(t, cloudformation.Ref(parameterTargetGroupTargetPortParameter).Integer(), tg.Port)
				require.Equal(t, cloudformation.String("HTTP"), tg.Protocol)
				require.Equal(t, cloudformation.String("HTTP"), tg.HealthCheckProtocol)

				validateTargetGroupListener(t, template, "TG", "HTTPListener", 80, "HTTP")
				validateTargetGroupListener(t, template, "TG", "HTTPSListener", 443, "HTTPS")
				validateTargetGroupOutput(t, template, "TG", "TargetGroupARN")
			},
		},
		{
			name: "http -> https redirect should be enabled for Application load balancers",
			spec: &stackSpec{
				loadbalancerType:    LoadBalancerTypeApplication,
				httpRedirectToHTTPS: true,
			},
			validate: func(t *testing.T, template *cloudformation.Template) {
				requireTargetGroups(t, template, "TG")
				requireListeners(t, template, "HTTPListener")
				listener := template.Resources["HTTPListener"].Properties.(*cloudformation.ElasticLoadBalancingV2Listener)
				require.Len(t, *listener.DefaultActions, 1)
				redirectConfig := []cloudformation.ElasticLoadBalancingV2ListenerAction(*listener.DefaultActions)[0].RedirectConfig
				require.NotNil(t, redirectConfig)
				require.Equal(t, redirectConfig.Protocol, cloudformation.String("HTTPS"))
				require.Equal(t, redirectConfig.StatusCode, cloudformation.String("HTTP_301"))
			},
		},
		{
			name: "http -> https redirect should NOT be enabled for Network load balancers",
			spec: &stackSpec{
				loadbalancerType:    LoadBalancerTypeNetwork,
				httpRedirectToHTTPS: true,
			},
			validate: func(t *testing.T, template *cloudformation.Template) {
				requireTargetGroups(t, template, "TG")
				requireListeners(t, template, "HTTPListener")
				listener := template.Resources["HTTPListener"].Properties.(*cloudformation.ElasticLoadBalancingV2Listener)
				require.Len(t, *listener.DefaultActions, 1)
				redirectConfig := []cloudformation.ElasticLoadBalancingV2ListenerAction(*listener.DefaultActions)[0].RedirectConfig
				require.Nil(t, redirectConfig)
			},
		},
		{
			name: "nlb cross zone load balancing can be enabled for Network load balancers",
			spec: &stackSpec{
				loadbalancerType: LoadBalancerTypeNetwork,
				nlbCrossZone:     true,
			},
			validate: func(t *testing.T, template *cloudformation.Template) {
				require.NotNil(t, template.Resources["LB"])
				properties := template.Resources["LB"].Properties.(*cloudformation.ElasticLoadBalancingV2LoadBalancer)
				attributes := []cloudformation.ElasticLoadBalancingV2LoadBalancerLoadBalancerAttribute(*properties.LoadBalancerAttributes)
				require.Equal(t, attributes[0].Key.Literal, "load_balancing.cross_zone.enabled")
				require.Equal(t, attributes[0].Value.Literal, "true")
			},
		},
		{
			name: "nlb HTTP listener should not be enabled when HTTP is disabled",
			spec: &stackSpec{
				loadbalancerType: LoadBalancerTypeNetwork,
				certificateARNs:  map[string]time.Time{"domain.company.com": time.Now()},
				httpDisabled:     true,
			},
			validate: func(t *testing.T, template *cloudformation.Template) {
				requireTargetGroups(t, template, "TG")
				requireListeners(t, template, "HTTPSListener")
			},
		},
		{
			name: "h2 should be enabled on ALB if set to true",
			spec: &stackSpec{
				loadbalancerType: LoadBalancerTypeApplication,
				http2:            true,
			},
			validate: func(t *testing.T, template *cloudformation.Template) {
				require.NotNil(t, template.Resources["LB"])
				properties := template.Resources["LB"].Properties.(*cloudformation.ElasticLoadBalancingV2LoadBalancer)
				attributes := []cloudformation.ElasticLoadBalancingV2LoadBalancerLoadBalancerAttribute(*properties.LoadBalancerAttributes)
				require.Equal(t, attributes[1].Key.Literal, "routing.http2.enabled")
				require.Equal(t, attributes[1].Value.Literal, "true")
				tg := template.Resources["TG"].Properties.(*cloudformation.ElasticLoadBalancingV2TargetGroup)
				require.Equal(t, cloudformation.String("HTTP1"), tg.ProtocolVersion)
			},
		},
		{
			name: "h2 should NOT be enabled on ALB if set to false",
			spec: &stackSpec{
				loadbalancerType: LoadBalancerTypeApplication,
				http2:            false,
			},
			validate: func(t *testing.T, template *cloudformation.Template) {
				require.NotNil(t, template.Resources["LB"])
				properties := template.Resources["LB"].Properties.(*cloudformation.ElasticLoadBalancingV2LoadBalancer)
				attributes := []cloudformation.ElasticLoadBalancingV2LoadBalancerLoadBalancerAttribute(*properties.LoadBalancerAttributes)
				require.Equal(t, attributes[1].Key.Literal, "routing.http2.enabled")
				require.Equal(t, attributes[1].Value.Literal, "false")
				tg := template.Resources["TG"].Properties.(*cloudformation.ElasticLoadBalancingV2TargetGroup)
				require.Equal(t, cloudformation.String("HTTP1"), tg.ProtocolVersion)
			},
		},
		{
			name: "stack has WAF Web ACL",
			spec: &stackSpec{
				loadbalancerType: LoadBalancerTypeApplication,
				wafWebAclId:      "foo-bar-baz",
			},
			validate: func(t *testing.T, template *cloudformation.Template) {
				require.NotNil(t, template.Resources["WAFAssociation"])
				props := template.Resources["WAFAssociation"].Properties.(*cloudformation.WAFRegionalWebACLAssociation)
				require.NotNil(t, props.ResourceArn)
				require.NotNil(t, props.WebACLID)
			},
		},
		{
			name: "deregistration timeout is set correctly",
			spec: &stackSpec{
				deregistrationDelayTimeoutSeconds: 1234,
			},
			validate: func(t *testing.T, template *cloudformation.Template) {
				require.NotNil(t, template.Resources["TG"])
				props := template.Resources["TG"].Properties.(*cloudformation.ElasticLoadBalancingV2TargetGroup)
				expected := cloudformation.ElasticLoadBalancingV2TargetGroupTargetGroupAttributeList{
					{
						Key:   cloudformation.String("deregistration_delay.timeout_seconds"),
						Value: cloudformation.String("1234"),
					},
				}
				require.Equal(t, &expected, props.TargetGroupAttributes)
			},
		},
		{
			name: "Does not set healthcheck timeout on NLBs",
			spec: &stackSpec{
				loadbalancerType: LoadBalancerTypeNetwork,
			},
			validate: func(t *testing.T, template *cloudformation.Template) {
				require.NotNil(t, template.Resources, "TG")
				tg, ok := template.Resources["TG"].Properties.(*cloudformation.ElasticLoadBalancingV2TargetGroup)
				require.True(t, ok, "couldn't convert resource to ElasticLoadBalancingV2TargetGroup")
				require.Nil(t, tg.HealthCheckTimeoutSeconds)
			},
		},
		{
			name: "Set healthcheck timeout when not NLB",
			spec: &stackSpec{
				loadbalancerType: LoadBalancerTypeApplication,
			},
			validate: func(t *testing.T, template *cloudformation.Template) {
				require.NotNil(t, template.Resources, "TG")
				tg, ok := template.Resources["TG"].Properties.(*cloudformation.ElasticLoadBalancingV2TargetGroup)
				require.True(t, ok, "couldn't convert resource to ElasticLoadBalancingV2TargetGroup")
				require.NotNil(t, tg.HealthCheckTimeoutSeconds)
			},
		},
		{
			name: "deny internal traffic for HTTPS correctly",
			spec: &stackSpec{
				loadbalancerType:    LoadBalancerTypeApplication,
				certificateARNs:     map[string]time.Time{"domain.company.com": time.Now()},
				httpRedirectToHTTPS: true,

				denyInternalDomains:         true,
				internalDomains:             internalDomains,
				denyInternalDomainsResponse: denyResp,
			},
			validate: func(t *testing.T, template *cloudformation.Template) {
				require.NotNil(t, template.Resources["HTTPSRuleBlockInternalTraffic"])
				resource := template.Resources["HTTPSRuleBlockInternalTraffic"]
				validateDenyRule(t, resource)
				require.NotContains(t, template.Resources, "HTTPRuleBlockInternalTraffic")
			},
		},
		{
			name: "deny internal traffic for HTTP correctly",
			spec: &stackSpec{
				loadbalancerType: LoadBalancerTypeApplication,

				denyInternalDomains:         true,
				internalDomains:             internalDomains,
				denyInternalDomainsResponse: denyResp,
			},
			validate: func(t *testing.T, template *cloudformation.Template) {
				require.NotNil(t, template.Resources["HTTPRuleBlockInternalTraffic"])
				resource := template.Resources["HTTPRuleBlockInternalTraffic"]
				validateDenyRule(t, resource)
				require.NotContains(t, template.Resources, "HTTPSRuleBlockInternalTraffic")
			},
		},
		{
			name: "deny internal traffic for HTTP/HTTPS correctly",
			spec: &stackSpec{
				loadbalancerType:    LoadBalancerTypeApplication,
				certificateARNs:     map[string]time.Time{"domain.company.com": time.Now()},
				httpRedirectToHTTPS: false,

				denyInternalDomains:         true,
				internalDomains:             internalDomains,
				denyInternalDomainsResponse: denyResp,
			},
			validate: func(t *testing.T, template *cloudformation.Template) {
				require.NotNil(t, template.Resources["HTTPRuleBlockInternalTraffic"])
				resource := template.Resources["HTTPRuleBlockInternalTraffic"]
				validateDenyRule(t, resource)

				require.NotNil(t, template.Resources["HTTPSRuleBlockInternalTraffic"])
				resource = template.Resources["HTTPSRuleBlockInternalTraffic"]
				validateDenyRule(t, resource)
			},
		},
		{
			name: "Does not deny internal traffic if not required",
			spec: &stackSpec{
				loadbalancerType:    LoadBalancerTypeApplication,
				certificateARNs:     map[string]time.Time{"domain.company.com": time.Now()},
				httpRedirectToHTTPS: false,

				denyInternalDomains: false,
			},
			validate: func(t *testing.T, template *cloudformation.Template) {
				require.NotContains(t, template.Resources, "HTTPSRuleBlockInternalTraffic")
				require.NotContains(t, template.Resources, "HTTPRuleBlockInternalTraffic")
			},
		},
		{
			name: "Does not create deny internal traffic rule on NLBs",
			spec: &stackSpec{
				loadbalancerType:    LoadBalancerTypeNetwork,
				certificateARNs:     map[string]time.Time{"domain.company.com": time.Now()},
				denyInternalDomains: true,
			},
			validate: func(t *testing.T, template *cloudformation.Template) {
				require.NotContains(t, template.Resources, "HTTPSRuleBlockInternalTraffic")
				require.NotContains(t, template.Resources, "HTTPRuleBlockInternalTraffic")
			},
		},
		{
			name: "Uses HTTPS targets when asked to",
			spec: &stackSpec{
				loadbalancerType: LoadBalancerTypeApplication,
				certificateARNs:  map[string]time.Time{"domain.company.com": time.Now()},
				targetHTTPS:      true,
			},
			validate: func(t *testing.T, template *cloudformation.Template) {
				requireTargetGroups(t, template, "TG")
				requireListeners(t, template, "HTTPListener", "HTTPSListener")

				tg := template.Resources["TG"].Properties.(*cloudformation.ElasticLoadBalancingV2TargetGroup)
				require.Equal(t, cloudformation.String("HTTPS"), tg.Protocol)
				require.Equal(t, cloudformation.String("HTTPS"), tg.HealthCheckProtocol)

				validateTargetGroupListener(t, template, "TG", "HTTPListener", 80, "HTTP")
				validateTargetGroupListener(t, template, "TG", "HTTPSListener", 443, "HTTPS")
			},
		},
		{
			name: "NLBs should always use the TCP protocol for listeners and TGs",
			spec: &stackSpec{
				loadbalancerType: LoadBalancerTypeNetwork,
				certificateARNs:  map[string]time.Time{"domain.company.com": time.Now()},
			},
			validate: func(t *testing.T, template *cloudformation.Template) {
				requireTargetGroups(t, template, "TG")
				requireListeners(t, template, "HTTPListener", "HTTPSListener")

				tg := template.Resources["TG"].Properties.(*cloudformation.ElasticLoadBalancingV2TargetGroup)
				require.Equal(t, cloudformation.String("TCP"), tg.Protocol)
				require.Equal(t, cloudformation.String("HTTP"), tg.HealthCheckProtocol)

				validateTargetGroupListener(t, template, "TG", "HTTPListener", 80, "TCP")
				validateTargetGroupListener(t, template, "TG", "HTTPSListener", 443, "TLS")
			},
		},
		{
			name: "NLBs should always use the TCP protocol for listeners and TGs - targetHTTPS should not impact it",
			spec: &stackSpec{
				loadbalancerType: LoadBalancerTypeNetwork,
				certificateARNs:  map[string]time.Time{"domain.company.com": time.Now()},
				targetHTTPS:      true,
			},
			validate: func(t *testing.T, template *cloudformation.Template) {
				requireTargetGroups(t, template, "TG")
				requireListeners(t, template, "HTTPListener", "HTTPSListener")

				tg := template.Resources["TG"].Properties.(*cloudformation.ElasticLoadBalancingV2TargetGroup)
				require.Equal(t, cloudformation.String("TCP"), tg.Protocol)
				require.Equal(t, cloudformation.String("HTTP"), tg.HealthCheckProtocol)

				validateTargetGroupListener(t, template, "TG", "HTTPListener", 80, "TCP")
				validateTargetGroupListener(t, template, "TG", "HTTPSListener", 443, "TLS")
			},
		},
		{
			name: "NLB should use separate Target Group for HTTP when HTTP target port is configured",
			spec: &stackSpec{
				loadbalancerType: LoadBalancerTypeNetwork,
				certificateARNs:  map[string]time.Time{"domain.company.com": time.Now()},
				httpTargetPort:   8888,
			},
			validate: func(t *testing.T, template *cloudformation.Template) {
				requireTargetGroups(t, template, "TGHTTP", "TG")
				requireListeners(t, template, "HTTPListener", "HTTPSListener")

				tgHTTP := template.Resources["TGHTTP"].Properties.(*cloudformation.ElasticLoadBalancingV2TargetGroup)
				require.Contains(t, template.Parameters, parameterTargetGroupHTTPTargetPortParameter)
				require.Equal(t, cloudformation.Ref(parameterTargetGroupHTTPTargetPortParameter).Integer(), tgHTTP.Port)
				require.Equal(t, cloudformation.String("TCP"), tgHTTP.Protocol)
				require.Equal(t, cloudformation.String("HTTP"), tgHTTP.HealthCheckProtocol)

				validateTargetGroupListener(t, template, "TGHTTP", "HTTPListener", 80, "TCP")
				validateTargetGroupOutput(t, template, "TGHTTP", "HTTPTargetGroupARN")

				tg := template.Resources["TG"].Properties.(*cloudformation.ElasticLoadBalancingV2TargetGroup)
				require.Contains(t, template.Parameters, parameterTargetGroupTargetPortParameter)
				require.Equal(t, cloudformation.Ref(parameterTargetGroupTargetPortParameter).Integer(), tg.Port)
				require.Equal(t, cloudformation.String("TCP"), tg.Protocol)
				require.Equal(t, cloudformation.String("HTTP"), tg.HealthCheckProtocol)
				require.Empty(t, tg.ProtocolVersion)

				validateTargetGroupListener(t, template, "TG", "HTTPSListener", 443, "TLS")
				validateTargetGroupOutput(t, template, "TG", "TargetGroupARN")
			},
		},
		{
			name: "NLB should use one Target Group when HTTP target port equals target port",
			spec: &stackSpec{
				loadbalancerType: LoadBalancerTypeNetwork,
				certificateARNs:  map[string]time.Time{"domain.company.com": time.Now()},
				targetPort:       9999,
				httpTargetPort:   9999,
			},
			validate: func(t *testing.T, template *cloudformation.Template) {
				requireTargetGroups(t, template, "TG")
				requireListeners(t, template, "HTTPListener", "HTTPSListener")

				tg := template.Resources["TG"].Properties.(*cloudformation.ElasticLoadBalancingV2TargetGroup)
				require.NotContains(t, template.Parameters, parameterTargetGroupHTTPTargetPortParameter)
				require.Contains(t, template.Parameters, parameterTargetGroupTargetPortParameter)
				require.Equal(t, cloudformation.Ref(parameterTargetGroupTargetPortParameter).Integer(), tg.Port)
				require.Equal(t, cloudformation.String("TCP"), tg.Protocol)
				require.Equal(t, cloudformation.String("HTTP"), tg.HealthCheckProtocol)

				validateTargetGroupListener(t, template, "TG", "HTTPListener", 80, "TCP")
				validateTargetGroupListener(t, template, "TG", "HTTPSListener", 443, "TLS")
				validateTargetGroupOutput(t, template, "TG", "TargetGroupARN")
			},
		},
		{
			name: "NLB HTTP target port is ignored when HTTP is disabled",
			spec: &stackSpec{
				loadbalancerType: LoadBalancerTypeNetwork,
				certificateARNs:  map[string]time.Time{"domain.company.com": time.Now()},
				httpDisabled:     true,
				httpTargetPort:   8888,
			},
			validate: func(t *testing.T, template *cloudformation.Template) {
				requireTargetGroups(t, template, "TG")
				requireListeners(t, template, "HTTPSListener")

				tg := template.Resources["TG"].Properties.(*cloudformation.ElasticLoadBalancingV2TargetGroup)
				require.NotContains(t, template.Parameters, parameterTargetGroupHTTPTargetPortParameter)
				require.Contains(t, template.Parameters, parameterTargetGroupTargetPortParameter)
				require.Equal(t, cloudformation.Ref(parameterTargetGroupTargetPortParameter).Integer(), tg.Port)
				require.Equal(t, cloudformation.String("TCP"), tg.Protocol)
				require.Equal(t, cloudformation.String("HTTP"), tg.HealthCheckProtocol)

				validateTargetGroupListener(t, template, "TG", "HTTPSListener", 443, "TLS")
				validateTargetGroupOutput(t, template, "TG", "TargetGroupARN")
			},
		},
		{
			name: "For Albs sets Healthy and Unhealthy Threshold Count individually",
			spec: &stackSpec{
				loadbalancerType:           LoadBalancerTypeApplication,
				albHealthyThresholdCount:   7,
				albUnhealthyThresholdCount: 3,
			},
			validate: func(t *testing.T, template *cloudformation.Template) {
				tg := template.Resources["TG"].Properties.(*cloudformation.ElasticLoadBalancingV2TargetGroup)
				require.Equal(t, cloudformation.Integer(7), tg.HealthyThresholdCount)
				require.Equal(t, cloudformation.Integer(3), tg.UnhealthyThresholdCount)
			},
		},
		{
			name: "For Nlbs sets Healthy and Unhealthy Threshold Count equally and ignores ALB settings",
			spec: &stackSpec{
				loadbalancerType:           LoadBalancerTypeNetwork,
				nlbHealthyThresholdCount:   4,
				albHealthyThresholdCount:   7,
				albUnhealthyThresholdCount: 3,
			},
			validate: func(t *testing.T, template *cloudformation.Template) {
				tg := template.Resources["TG"].Properties.(*cloudformation.ElasticLoadBalancingV2TargetGroup)
				require.Equal(t, cloudformation.Integer(4), tg.HealthyThresholdCount)
				require.Equal(t, cloudformation.Integer(4), tg.UnhealthyThresholdCount)
				require.NotEqual(t, cloudformation.Integer(7), tg.HealthyThresholdCount)
				require.NotEqual(t, cloudformation.Integer(3), tg.UnhealthyThresholdCount)
			},
		},
		{
			name: "target port http2 when https listener and configured",
			spec: &stackSpec{
				loadbalancerType:           LoadBalancerTypeApplication,
				http2:                      true,
				targetHTTPS:                true,
				targetGroupProtocolVersion: "HTTP2",
			},
			validate: func(t *testing.T, template *cloudformation.Template) {
				require.NotNil(t, template.Resources["LB"])
				properties := template.Resources["LB"].Properties.(*cloudformation.ElasticLoadBalancingV2LoadBalancer)
				attributes := []cloudformation.ElasticLoadBalancingV2LoadBalancerLoadBalancerAttribute(*properties.LoadBalancerAttributes)
				require.Equal(t, attributes[1].Key.Literal, "routing.http2.enabled")
				require.Equal(t, attributes[1].Value.Literal, "true")
				tg := template.Resources["TG"].Properties.(*cloudformation.ElasticLoadBalancingV2TargetGroup)
				require.Equal(t, cloudformation.String("HTTP2"), tg.ProtocolVersion)
			},
		},
		{
			name: "For Nlbs target protocol is undefined",
			spec: &stackSpec{
				loadbalancerType:           LoadBalancerTypeNetwork,
				http2:                      true,
				targetGroupProtocolVersion: "HTTP2",
				targetHTTPS:                true,
			},
			validate: func(t *testing.T, template *cloudformation.Template) {
				require.NotNil(t, template.Resources["LB"])
				tg := template.Resources["TG"].Properties.(*cloudformation.ElasticLoadBalancingV2TargetGroup)
				require.Nil(t, tg.ProtocolVersion)
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			generated, err := generateTemplate(test.spec)

			require.NoError(t, err)

			var template *cloudformation.Template

			err = json.Unmarshal([]byte(generated), &template)

			require.NoError(t, err)

			test.validate(t, template)
		})
	}
}

func validateTargetGroupListener(t *testing.T, template *cloudformation.Template, targetGroup string, name string, port int64, protocol string) {
	resource, ok := template.Resources[name]
	require.True(t, ok, "Resource %s expected", name)

	listener, ok := resource.Properties.(*cloudformation.ElasticLoadBalancingV2Listener)
	require.True(t, ok, "Wrong type")
	require.Equal(t, cloudformation.String("forward"), (*listener.DefaultActions)[0].Type)
	require.Equal(t, cloudformation.Ref(targetGroup).String(), (*listener.DefaultActions)[0].TargetGroupArn)
	require.Equal(t, cloudformation.Integer(port), listener.Port)
	require.Equal(t, cloudformation.String(protocol), listener.Protocol)
}

func validateTargetGroupOutput(t *testing.T, template *cloudformation.Template, targetGroup, name string) {
	output, ok := template.Outputs[name]
	require.True(t, ok, "Resource %s expected", name)
	require.Equal(t, map[string]interface{}{"Ref": targetGroup}, output.Value)
}

func requireTargetGroups(t *testing.T, template *cloudformation.Template, expected ...string) {
	requireResources(t, template, expected, func(v interface{}) bool {
		_, ok := v.(*cloudformation.ElasticLoadBalancingV2TargetGroup)
		return ok
	})
}

func requireListeners(t *testing.T, template *cloudformation.Template, expected ...string) {
	requireResources(t, template, expected, func(v interface{}) bool {
		_, ok := v.(*cloudformation.ElasticLoadBalancingV2Listener)
		return ok
	})
}

func requireResources(t *testing.T, template *cloudformation.Template, expected []string, matches func(interface{}) bool) {
	var names []string
	for name, resource := range template.Resources {
		if matches(resource.Properties) {
			names = append(names, name)
		}
	}
	require.ElementsMatch(t, expected, names)
}
