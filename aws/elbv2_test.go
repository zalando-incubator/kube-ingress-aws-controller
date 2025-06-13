package aws

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2Types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

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

func TestGetLoadBalancer(t *testing.T) {
	for _, ti := range []struct {
		name           string
		givenCFOutput  fake.CFOutputs
		givenELBOutput fake.ELBv2Outputs
		wantELB        *elbv2Types.LoadBalancer
		wantError      bool
	}{
		{
			"one active managed load balancer",
			fake.CFOutputs{
				ListStackResources: fake.R(
					&cloudformation.ListStackResourcesOutput{
						StackResourceSummaries: []types.StackResourceSummary{
							{
								LogicalResourceId:  aws.String(loadBalancerResourceLogicalID),
								ResourceType:       aws.String("AWS::ElasticLoadBalancingV2::LoadBalancer"),
								PhysicalResourceId: aws.String("myELB"),
							},
						},
					}, nil),
			},
			fake.ELBv2Outputs{
				DescribeLoadBalancers: fake.R(
					&elbv2.DescribeLoadBalancersOutput{
						LoadBalancers: []elbv2Types.LoadBalancer{
							{
								LoadBalancerArn: aws.String("aws-arn-with-myELB-physical-id"),
								State: &elbv2Types.LoadBalancerState{
									Code:   elbv2Types.LoadBalancerStateEnumActive,
									Reason: aws.String(""),
								},
							},
						},
					}, nil),
			},
			&elbv2Types.LoadBalancer{
				LoadBalancerArn: aws.String("aws-arn-with-myELB-physical-id"),
				State: &elbv2Types.LoadBalancerState{
					Code:   elbv2Types.LoadBalancerStateEnumActive,
					Reason: aws.String(""),
				},
			},
			false,
		},
		{
			"error when listing aws stack resources",
			fake.CFOutputs{
				ListStackResources: fake.R(nil, fake.ErrDummy),
			},
			fake.ELBv2Outputs{
				DescribeLoadBalancers: fake.R(nil, nil),
			},
			nil,
			true,
		},
		{
			"error when describing aws load balancers",
			fake.CFOutputs{
				ListStackResources: fake.R(
					&cloudformation.ListStackResourcesOutput{
						StackResourceSummaries: []types.StackResourceSummary{
							{
								LogicalResourceId:  aws.String(loadBalancerResourceLogicalID),
								ResourceType:       aws.String("AWS::ElasticLoadBalancingV2::LoadBalancer"),
								PhysicalResourceId: aws.String("myELB"),
							},
						},
					}, nil),
			},
			fake.ELBv2Outputs{
				DescribeLoadBalancers: fake.R(nil, fake.ErrDummy),
			},
			nil,
			true,
		},
		{
			"stack has load balancer resource but no load balancer",
			fake.CFOutputs{
				ListStackResources: fake.R(
					&cloudformation.ListStackResourcesOutput{
						StackResourceSummaries: []types.StackResourceSummary{
							{
								LogicalResourceId:  aws.String(loadBalancerResourceLogicalID),
								ResourceType:       aws.String("AWS::ElasticLoadBalancingV2::LoadBalancer"),
								PhysicalResourceId: aws.String("myELB"),
							},
						},
					}, nil),
			},
			fake.ELBv2Outputs{
				DescribeLoadBalancers: fake.R(
					&elbv2.DescribeLoadBalancersOutput{
						LoadBalancers: []elbv2Types.LoadBalancer{},
					}, nil),
			},
			nil,
			true,
		},
		{
			"multiple load balancers",
			fake.CFOutputs{
				ListStackResources: fake.R(
					&cloudformation.ListStackResourcesOutput{
						StackResourceSummaries: []types.StackResourceSummary{
							{
								LogicalResourceId:  aws.String(loadBalancerResourceLogicalID),
								ResourceType:       aws.String("AWS::ElasticLoadBalancingV2::LoadBalancer"),
								PhysicalResourceId: aws.String("myELB"),
							},
						},
					}, nil),
			},
			fake.ELBv2Outputs{
				DescribeLoadBalancers: fake.R(
					&elbv2.DescribeLoadBalancersOutput{
						LoadBalancers: []elbv2Types.LoadBalancer{
							{
								LoadBalancerArn: aws.String("aws-arn-with-myELB-physical-id-002"),
								State: &elbv2Types.LoadBalancerState{
									Code:   elbv2Types.LoadBalancerStateEnumActive,
									Reason: aws.String(""),
								},
							},
							{
								LoadBalancerArn: aws.String("aws-arn-with-myELB-physical-id-001"),
								State: &elbv2Types.LoadBalancerState{
									Code:   elbv2Types.LoadBalancerStateEnumFailed,
									Reason: aws.String("cannot create the load balancer"),
								},
							},
						},
					}, nil),
			},
			nil,
			true,
		},
	} {
		t.Run(ti.name, func(t *testing.T) {
			svc := &fake.ELBv2Client{Outputs: ti.givenELBOutput}
			cfSvc := &fake.CFClient{Outputs: ti.givenCFOutput}

			a := Adapter{
				elbv2:          svc,
				cloudformation: cfSvc,
			}

			elb, err := a.GetLoadBalancer(context.Background(), "myELB")
			if ti.wantError && err == nil {
				t.Fatalf("expected error, got nothing")
			}
			if !ti.wantError && err != nil {
				t.Fatalf("unexpected error - %q", err)
			}
			if !ti.wantError && !reflect.DeepEqual(elb, ti.wantELB) {
				t.Errorf("unexpected ELB. expected: %v, got: %v", ti.wantELB, elb)
			}
		},
		)
	}

}
