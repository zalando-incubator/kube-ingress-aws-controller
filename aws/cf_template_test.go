package aws

import (
	"encoding/json"
	"testing"

	cloudformation "github.com/mweagle/go-cloudformation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateTemplate(t *testing.T) {
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
			name: "http -> https redirect should be enabled for Application load balancers",
			spec: &stackSpec{
				loadbalancerType:    LoadBalancerTypeApplication,
				httpRedirectToHTTPS: true,
			},
			validate: func(t *testing.T, template *cloudformation.Template) {
				require.NotNil(t, template.Resources["HTTPListener"])
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
				nlbHTTPEnabled:      true,
			},
			validate: func(t *testing.T, template *cloudformation.Template) {
				require.NotNil(t, template.Resources["HTTPListener"])
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
			name: "nlb HTTP listener should not be enabled when nlbHTTPEnabled is set to false",
			spec: &stackSpec{
				loadbalancerType: LoadBalancerTypeNetwork,
				nlbHTTPEnabled:   false,
			},
			validate: func(t *testing.T, template *cloudformation.Template) {
				require.NotContains(t, template.Resources, "HTTPListener")
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
