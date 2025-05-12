package aws

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/zalando-incubator/kube-ingress-aws-controller/aws/fake"
)

type registerTargetsOnTargetGroupsInputTest struct {
	targetGroupARNs []string
	instances       []string
}

type deregisterTargetsOnTargetGroupsInputTest struct {
	targetGroupARNs []string
	instances       []string
}

func TestRegisterTargetsOnTargetGroups(t *testing.T) {
	outputsSuccess := fake.ELBv2Outputs{
		RegisterTargets: fake.R(fake.MockRTOutput(), nil),
	}
	outputsError := fake.ELBv2Outputs{
		RegisterTargets: fake.R(fake.MockRTOutput(), fake.ErrDummy),
	}

	for _, test := range []struct {
		name      string
		input     registerTargetsOnTargetGroupsInputTest
		outputs   fake.ELBv2Outputs
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
			svc := &fake.ELBv2Client{Outputs: test.outputs}
			err := registerTargetsOnTargetGroups(context.Background(), svc, test.input.targetGroupARNs, test.input.instances)
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
				for _, input := range svc.Rtinputs {
					rtTargetsGroupARNs = append(rtTargetsGroupARNs, aws.ToString(input.TargetGroupArn))
					rtInstances := make([]string, len(input.Targets))
					for j, tgt := range input.Targets {
						rtInstances[j] = aws.ToString(tgt.Id)
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

func TestDeregisterTargetsOnTargetGroups(t *testing.T) {
	outputsSuccess := fake.ELBv2Outputs{
		DeregisterTargets: fake.R(fake.MockDeregisterTargetsOutput(), nil),
	}
	outputsError := fake.ELBv2Outputs{
		DeregisterTargets: fake.R(fake.MockDeregisterTargetsOutput(), fake.ErrDummy),
	}

	for _, test := range []struct {
		name      string
		input     deregisterTargetsOnTargetGroupsInputTest
		outputs   fake.ELBv2Outputs
		wantError bool
	}{
		{
			"single-target-group",
			deregisterTargetsOnTargetGroupsInputTest{
				targetGroupARNs: []string{"asg1"},
				instances:       []string{"i1", "i2"},
			},
			outputsSuccess,
			false,
		},
		{
			"multiple-target-groups",
			deregisterTargetsOnTargetGroupsInputTest{
				targetGroupARNs: []string{"asg1", "asg2"},
				instances:       []string{"i1", "i2"},
			},
			outputsSuccess,
			false,
		},
		{
			"empty-input-no-error",
			deregisterTargetsOnTargetGroupsInputTest{
				targetGroupARNs: []string{},
				instances:       []string{},
			},
			outputsSuccess,
			false,
		},
		{
			"error1",
			deregisterTargetsOnTargetGroupsInputTest{
				targetGroupARNs: []string{"asg1"},
				instances:       []string{"i1", "i2"},
			},
			outputsError,
			true,
		},
		{
			"error2",
			deregisterTargetsOnTargetGroupsInputTest{
				targetGroupARNs: []string{"asg1", "asg2"},
				instances:       []string{"i1", "i2"},
			},
			outputsError,
			true,
		},
	} {
		t.Run(fmt.Sprintf("%v", test.name), func(t *testing.T) {
			svc := &fake.ELBv2Client{Outputs: test.outputs}
			err := deregisterTargetsOnTargetGroups(context.Background(), svc, test.input.targetGroupARNs, test.input.instances)
			if test.wantError && err == nil {
				t.Fatalf("expected error, got nothing")
			}
			if !test.wantError && err != nil {
				t.Fatalf("unexpected error - %q", err)
			}
			if !test.wantError && err == nil {
				sort.Strings(test.input.targetGroupARNs)
				sort.Strings(test.input.instances)
				dtTargetsGroupARNs := make([]string, 0, len(test.input.targetGroupARNs))
				for _, input := range svc.Dtinputs {
					dtTargetsGroupARNs = append(dtTargetsGroupARNs, aws.ToString(input.TargetGroupArn))
					dtInstances := make([]string, len(input.Targets))
					for j, tgt := range input.Targets {
						dtInstances[j] = aws.ToString(tgt.Id)
					}
					sort.Strings(dtInstances)
					if !reflect.DeepEqual(dtInstances, test.input.instances) {
						t.Errorf("unexpected set of registered instances. expected: %q, got: %q", test.input.instances, dtInstances)
					}
				}
				sort.Strings(dtTargetsGroupARNs)
				if !reflect.DeepEqual(dtTargetsGroupARNs, test.input.targetGroupARNs) {
					t.Errorf("unexpected set of targetGroupARNs. expected: %q, got: %q", test.input.targetGroupARNs, dtTargetsGroupARNs)
				}
			}
		})
	}
}
