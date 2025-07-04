package aws

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"

	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2Types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/zalando-incubator/kube-ingress-aws-controller/aws/fake"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type registerTargetsOnTargetGroupsInputTest struct {
	targetGroupARNs []string
	instances       []string
}

type deregisterTargetsOnTargetGroupsInputTest struct {
	targetGroupARNs []string
	instances       []string
}

func TestIsActiveLBState(t *testing.T) {
	tests := []struct {
		name     string
		lbState  *LoadBalancerState
		expected bool
	}{
		{"nil state", nil, false},
		{"active state", &LoadBalancerState{StateCode: elbv2Types.LoadBalancerStateEnumActive}, true},
		{"provisioning state", &LoadBalancerState{StateCode: elbv2Types.LoadBalancerStateEnumProvisioning}, false},
		{"active impaired state", &LoadBalancerState{StateCode: elbv2Types.LoadBalancerStateEnumActiveImpaired}, false},
		{"provisioning state", &LoadBalancerState{StateCode: elbv2Types.LoadBalancerStateEnumFailed}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsActiveLBState(tt.lbState)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetLBStateString(t *testing.T) {
	tests := []struct {
		name     string
		lbState  *LoadBalancerState
		expected string
	}{
		{"nil state", nil, "nil"},
		{"active state", &LoadBalancerState{StateCode: elbv2Types.LoadBalancerStateEnumActive}, "active"},
		{"provisioning state", &LoadBalancerState{StateCode: elbv2Types.LoadBalancerStateEnumProvisioning}, "provisioning"},
		{"active impaired state", &LoadBalancerState{StateCode: elbv2Types.LoadBalancerStateEnumActiveImpaired}, "active_impaired"},
		{"failed state", &LoadBalancerState{StateCode: elbv2Types.LoadBalancerStateEnumFailed}, "failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetLBStateString(tt.lbState)
			assert.Equal(t, tt.expected, result)
		})
	}
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

func TestGetLoadBalancerStates(t *testing.T) {
	outputBasedOnInput := fake.ELBv2Outputs{DescribeLoadBalancers: nil}
	expectedLBState := LoadBalancerState{
		StateCode: elbv2Types.LoadBalancerStateEnumActive,
		Reason:    "Mocked state",
	}

	for _, test := range []struct {
		name           string
		input          []string
		outputs        fake.ELBv2Outputs
		expectedOutput map[string]*LoadBalancerState
		wantError      bool
	}{
		{
			"single load balancer",
			[]string{"lb1"},
			outputBasedOnInput,
			map[string]*LoadBalancerState{
				"lb1": &expectedLBState,
			},
			false,
		},
		{
			"multiple load balancers",
			[]string{"lb1", "lb2", "lb3", "lb4"},
			outputBasedOnInput,
			map[string]*LoadBalancerState{
				"lb1": &expectedLBState,
				"lb2": &expectedLBState,
				"lb3": &expectedLBState,
				"lb4": &expectedLBState,
			},
			false,
		},
		{
			"more than 20 load balancers",
			[]string{
				"lb1", "lb2", "lb3", "lb4", "lb5", "lb6", "lb7", "lb8", "lb9", "lb10",
				"lb11", "lb12", "lb13", "lb14", "lb15", "lb16", "lb17", "lb18", "lb19",
				"lb20", "lb21", "lb22",
			},
			outputBasedOnInput,
			map[string]*LoadBalancerState{
				"lb1":  &expectedLBState,
				"lb2":  &expectedLBState,
				"lb3":  &expectedLBState,
				"lb4":  &expectedLBState,
				"lb5":  &expectedLBState,
				"lb6":  &expectedLBState,
				"lb7":  &expectedLBState,
				"lb8":  &expectedLBState,
				"lb9":  &expectedLBState,
				"lb10": &expectedLBState,
				"lb11": &expectedLBState,
				"lb12": &expectedLBState,
				"lb13": &expectedLBState,
				"lb14": &expectedLBState,
				"lb15": &expectedLBState,
				"lb16": &expectedLBState,
				"lb17": &expectedLBState,
				"lb18": &expectedLBState,
				"lb19": &expectedLBState,
				"lb20": &expectedLBState,
				"lb21": &expectedLBState,
				"lb22": &expectedLBState,
			},
			false,
		},
		{
			"no load balancers",
			[]string{},
			outputBasedOnInput,
			map[string]*LoadBalancerState{},
			false,
		},
		{
			"error response",
			[]string{"lb1"},
			fake.ELBv2Outputs{DescribeLoadBalancers: fake.R(nil, fake.ErrDummy)},
			nil,
			true,
		},
		{
			"nil load balancer state",
			[]string{"lb1"},
			fake.ELBv2Outputs{DescribeLoadBalancers: fake.R(&elbv2.DescribeLoadBalancersOutput{
				LoadBalancers: []elbv2Types.LoadBalancer{
					{
						LoadBalancerArn: aws.String("lb1"),
						State:           nil, // No state information
					},
				}}, nil)},
			map[string]*LoadBalancerState{
				"lb1": nil, // Expecting nil state for lb1
			},
			false,
		},
	} {
		t.Run(fmt.Sprintf("%v", test.name), func(t *testing.T) {
			svc := &fake.ELBv2Client{Outputs: test.outputs}
			states, err := getLoadBalancerStates(context.Background(), svc, test.input)
			if test.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, test.expectedOutput, states)
			}
		})
	}
}
