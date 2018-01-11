package aws

import (
	"fmt"
	"reflect"
	"sort"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
)

type registerTargetsOnTargetGroupsInputTest struct {
	targetGroupARNs []string
	instances       []string
}

func TestRegisterTargetsOnTargetGroups(t *testing.T) {
	outputsSuccess := elbv2MockOutputs{
		registerTargets: R(mockRTOutput(), nil),
	}
	outputsError := elbv2MockOutputs{
		registerTargets: R(mockRTOutput(), dummyErr),
	}

	for _, test := range []struct {
		name      string
		input     registerTargetsOnTargetGroupsInputTest
		outputs   elbv2MockOutputs
		wantError bool
	}{
		{
			"single-target-group",
			registerTargetsOnTargetGroupsInputTest{
				targetGroupARNs: []string{"asg1"},
				instances:       []string{"i1", "i2"},
			},
			outputsSuccess,
			false,
		},
		{
			"multiple-target-groups",
			registerTargetsOnTargetGroupsInputTest{
				targetGroupARNs: []string{"asg1", "asg2"},
				instances:       []string{"i1", "i2"},
			},
			outputsSuccess,
			false,
		},
		{
			"empty-input-no-error",
			registerTargetsOnTargetGroupsInputTest{
				targetGroupARNs: []string{},
				instances:       []string{},
			},
			outputsSuccess,
			false,
		},
		{
			"error1",
			registerTargetsOnTargetGroupsInputTest{
				targetGroupARNs: []string{"asg1"},
				instances:       []string{"i1", "i2"},
			},
			outputsError,
			true,
		},
		{
			"error2",
			registerTargetsOnTargetGroupsInputTest{
				targetGroupARNs: []string{"asg1", "asg2"},
				instances:       []string{"i1", "i2"},
			},
			outputsError,
			true,
		},
	} {
		t.Run(fmt.Sprintf("%v", test.name), func(t *testing.T) {
			svc := &mockElbv2Client{outputs: test.outputs}
			err := registerTargetsOnTargetGroups(svc, test.input.targetGroupARNs, test.input.instances)
			if test.wantError && err == nil {
				t.Fatalf("expected error, got nothing")
			}
			if !test.wantError && err != nil {
				t.Fatalf("unexpected error - %q", err)
			}
			if !test.wantError {
				sort.Strings(test.input.targetGroupARNs)
				sort.Strings(test.input.instances)
				rtTargetsGroupARNs := make([]string, 0, len(test.input.targetGroupARNs))
				for _, input := range svc.rtinputs {
					rtTargetsGroupARNs = append(rtTargetsGroupARNs, aws.StringValue(input.TargetGroupArn))
					rtInstances := make([]string, len(input.Targets))
					for j, tgt := range input.Targets {
						rtInstances[j] = aws.StringValue(tgt.Id)
					}
					sort.Strings(rtInstances)
					if !reflect.DeepEqual(rtInstances, test.input.instances) {
						t.Errorf("unexpected set of registered instances. expected: %q, got: %q", test.input.instances, rtInstances)
					}
				}
				sort.Strings(rtTargetsGroupARNs)
				if !reflect.DeepEqual(rtTargetsGroupARNs, test.input.targetGroupARNs) {
					t.Errorf("unexpected set of targetGroupARNs. expected: %q, got: %q", test.input.targetGroupARNs, rtTargetsGroupARNs)
				}
			}
		})
	}
}
