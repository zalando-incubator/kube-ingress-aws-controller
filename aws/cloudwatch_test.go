package aws

import (
	"testing"

	cloudformation "github.com/o11n/go-cloudformation"
	"github.com/stretchr/testify/assert"
)

func TestCloudWatchAlarmList_Hash(t *testing.T) {
	for _, test := range []struct {
		name     string
		list     CloudWatchAlarmList
		expected string
	}{
		{
			name:     "nil list produces empty hash",
			list:     nil,
			expected: "",
		},
		{
			name:     "empty list produces empty hash",
			list:     CloudWatchAlarmList{},
			expected: "",
		},
		{
			name: "list produces hash",
			list: CloudWatchAlarmList{
				{},
			},
			expected: "e10808d43975dc400731053386849f864f297e6c4f7519c380f3dbaf7067a840",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, test.list.Hash())
		})
	}
}

func TestNormalizeCloudWatchAlarmName(t *testing.T) {
	for _, test := range []struct {
		name      string
		alarmName *cloudformation.StringExpr
		expected  *cloudformation.StringExpr
	}{
		{
			name:      "prefix name with stack name if set",
			alarmName: cloudformation.String("foo-alarm"),
			expected: cloudformation.Join(
				"-",
				cloudformation.Ref("AWS::StackName"),
				cloudformation.String("foo-alarm"),
			),
		},
		{
			name:      "nil gets just passed through",
			alarmName: nil,
			expected:  nil,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := normalizeCloudWatchAlarmName(test.alarmName)

			assert.Equal(t, test.expected, result)
		})
	}
}

func TestNormalizeCloudWatchAlarmNamespace(t *testing.T) {
	for _, test := range []struct {
		name           string
		alarmNamespace *cloudformation.StringExpr
		expected       *cloudformation.StringExpr
	}{
		{
			name:           "namespace will not be altered if set",
			alarmNamespace: cloudformation.String("AWS/NetworkELB"),
			expected:       cloudformation.String("AWS/NetworkELB"),
		},
		{
			name:           "namespace is set to AWS/ApplicationELB if nil",
			alarmNamespace: nil,
			expected:       cloudformation.String("AWS/ApplicationELB"),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := normalizeCloudWatchAlarmNamespace(test.alarmNamespace)

			assert.Equal(t, test.expected, result)
		})
	}
}

func TestNormalizeCloudWatchAlarmDimensions(t *testing.T) {
	for _, test := range []struct {
		name            string
		alarmDimensions *cloudformation.CloudWatchAlarmDimensionList
		expected        *cloudformation.CloudWatchAlarmDimensionList
	}{
		{
			name:            "set to default if nil",
			alarmDimensions: nil,
			expected: &cloudformation.CloudWatchAlarmDimensionList{
				{
					Name:  cloudformation.String("LoadBalancer"),
					Value: cloudformation.GetAtt("LB", "LoadBalancerFullName").String(),
				},
			},
		},
		{
			name:            "set to default if empty",
			alarmDimensions: &cloudformation.CloudWatchAlarmDimensionList{},
			expected: &cloudformation.CloudWatchAlarmDimensionList{
				{
					Name:  cloudformation.String("LoadBalancer"),
					Value: cloudformation.GetAtt("LB", "LoadBalancerFullName").String(),
				},
			},
		},
		{
			name: "replaces LoadBalancer and TargetGroup with references",
			alarmDimensions: &cloudformation.CloudWatchAlarmDimensionList{
				{
					Name:  cloudformation.String("LoadBalancer"),
					Value: cloudformation.String("foo-lb"),
				},
				{
					Name: cloudformation.String("TargetGroup"),
				},
				{
					Name:  cloudformation.String("AvailabilityZone"),
					Value: cloudformation.String("eu-west-1a"),
				},
			},
			expected: &cloudformation.CloudWatchAlarmDimensionList{
				{
					Name:  cloudformation.String("LoadBalancer"),
					Value: cloudformation.GetAtt("LB", "LoadBalancerFullName").String(),
				},
				{
					Name:  cloudformation.String("TargetGroup"),
					Value: cloudformation.GetAtt("TG", "TargetGroupFullName").String(),
				},
				{
					Name:  cloudformation.String("AvailabilityZone"),
					Value: cloudformation.String("eu-west-1a"),
				},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := normalizeCloudWatchAlarmDimensions(test.alarmDimensions)

			assert.Equal(t, test.expected, result)
		})
	}
}
