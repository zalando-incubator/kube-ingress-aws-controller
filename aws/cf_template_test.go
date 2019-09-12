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
