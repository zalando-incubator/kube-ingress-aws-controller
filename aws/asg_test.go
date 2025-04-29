package aws

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	"github.com/stretchr/testify/assert"

	"github.com/zalando-incubator/kube-ingress-aws-controller/aws/fake"
)

func mockAutoScalingGroupDetails(name string, tags map[string]string) *autoScalingGroupDetails {
	return &autoScalingGroupDetails{
		name:         name,
		targetGroups: []string{},
		tags:         tags,
	}
}

func TestGetAutoScalingGroupByName(t *testing.T) {
	for _, test := range []struct {
		name      string
		givenName string
		responses fake.ASGOutputs
		want      *autoScalingGroupDetails
		wantError bool
	}{
		{
			"success-call-single-asg",
			"foo",
			fake.ASGOutputs{
				DescribeAutoScalingGroups: fake.R(fake.MockDescribeAutoScalingGroupOutput(map[string]fake.ASGtags{"foo": {"bar": "baz"}}), nil),
			},
			mockAutoScalingGroupDetails("foo", map[string]string{"bar": "baz"}),
			false,
		},
		{
			"success-call-multiple-asg",
			"d",
			fake.ASGOutputs{
				DescribeAutoScalingGroups: fake.R(fake.MockDescribeAutoScalingGroupOutput(map[string]fake.ASGtags{
					"a": {"b": "c"},
					"d": {"e": "f"},
				}), nil),
			},
			mockAutoScalingGroupDetails("d", map[string]string{"e": "f"}),
			false,
		},
		{
			"fail-to-match-single-asg",
			"miss",
			fake.ASGOutputs{
				DescribeAutoScalingGroups: fake.R(fake.MockDescribeAutoScalingGroupOutput(map[string]fake.ASGtags{"foo": {"bar": "baz"}}), nil),
			},
			nil,
			true,
		},
		{
			"fail-to-match-multiple-asg",
			"miss",
			fake.ASGOutputs{
				DescribeAutoScalingGroups: fake.R(fake.MockDescribeAutoScalingGroupOutput(map[string]fake.ASGtags{
					"a": {"b": "c"},
					"d": {"e": "f"},
				}), nil),
			},
			nil,
			true,
		},
		{
			"autoscaling-api-failure",
			"dontcare",
			fake.ASGOutputs{DescribeAutoScalingGroups: fake.R(nil, fake.ErrDummy)},
			nil,
			true,
		},
	} {
		t.Run(fmt.Sprintf("%v", test.name), func(t *testing.T) {
			mockSvc := &fake.ASGClient{Outputs: test.responses}
			got, err := getAutoScalingGroupByName(context.Background(), mockSvc, test.givenName)

			if test.wantError {
				if err == nil {
					t.Error("wanted an error but call seemed to have succeeded")
				}
			} else {
				if err != nil {
					t.Fatal("unexpected error", err)
				}
				if !reflect.DeepEqual(test.want, got) {
					t.Errorf("unexpected result. wanted %+v, got %+v", test.want, got)
				}
			}
		})
	}
}

func TestGetAutoScalingGroupsByName(t *testing.T) {
	for _, test := range []struct {
		name       string
		givenNames []string
		responses  fake.ASGOutputs
		want       map[string]*autoScalingGroupDetails
		wantError  bool
	}{
		{
			"success-call-single-asg",
			[]string{"foo"},
			fake.ASGOutputs{
				DescribeAutoScalingGroups: fake.R(fake.MockDescribeAutoScalingGroupOutput(map[string]fake.ASGtags{"foo": {"bar": "baz"}}), nil),
			},
			map[string]*autoScalingGroupDetails{
				"foo": mockAutoScalingGroupDetails("foo", map[string]string{"bar": "baz"}),
			},
			false,
		},
		{
			"success-call-multiple-asg",
			[]string{"a", "d"},
			fake.ASGOutputs{
				DescribeAutoScalingGroups: fake.R(fake.MockDescribeAutoScalingGroupOutput(map[string]fake.ASGtags{
					"a": {"b": "c"},
					"d": {"e": "f"},
				}), nil),
			},
			map[string]*autoScalingGroupDetails{
				"a": mockAutoScalingGroupDetails("a", map[string]string{"b": "c"}),
				"d": mockAutoScalingGroupDetails("d", map[string]string{"e": "f"}),
			},
			false,
		},
		{
			"fail-to-match-single-asg",
			[]string{"miss"},
			fake.ASGOutputs{
				DescribeAutoScalingGroups: fake.R(fake.MockDescribeAutoScalingGroupOutput(map[string]fake.ASGtags{"foo": {"bar": "baz"}}), nil),
			},
			nil,
			true,
		},
		{
			"fail-to-match-multiple-asg",
			[]string{"miss", "miss2"},
			fake.ASGOutputs{
				DescribeAutoScalingGroups: fake.R(fake.MockDescribeAutoScalingGroupOutput(map[string]fake.ASGtags{
					"a": {"b": "c"},
					"d": {"e": "f"},
				}), nil),
			},
			nil,
			true,
		},
		{
			"autoscaling-api-failure",
			[]string{"dontcare"},
			fake.ASGOutputs{DescribeAutoScalingGroups: fake.R(nil, fake.ErrDummy)},
			nil,
			true,
		},
	} {
		t.Run(fmt.Sprintf("%v", test.name), func(t *testing.T) {
			mockSvc := &fake.ASGClient{Outputs: test.responses}
			got, err := getAutoScalingGroupsByName(context.Background(), mockSvc, test.givenNames)

			if test.wantError {
				if err == nil {
					t.Error("wanted an error but call seemed to have succeeded")
				}
			} else {
				if err != nil {
					t.Fatal("unexpected error", err)
				}
				if !reflect.DeepEqual(test.want, got) {
					t.Errorf("unexpected result. wanted %v, got %v", test.want, got)
				}
			}
		})
	}
}

func TestAttach(t *testing.T) {
	for _, test := range []struct {
		name               string
		targetGroups       []string
		autoscalingOutputs fake.ASGOutputs
		autoscalingInputs  fake.ASGInputs
		elbv2Response      fake.ELBv2Outputs
		ownerTags          map[string]string
		wantError          bool
	}{
		{
			name:         "describe-lb-target-groups-failed",
			targetGroups: []string{"foo"},
			autoscalingOutputs: fake.ASGOutputs{
				DescribeLoadBalancerTargetGroups: fake.R(nil, fake.ErrDummy),
			},
			wantError: true,
		},
		{
			name:         "describe-all-target-groups-failed",
			targetGroups: []string{"foo"},
			autoscalingOutputs: fake.ASGOutputs{
				AttachLoadBalancerTargetGroups: fake.R(nil, nil),
				DescribeLoadBalancerTargetGroups: fake.R(&autoscaling.DescribeLoadBalancerTargetGroupsOutput{
					LoadBalancerTargetGroups: []types.LoadBalancerTargetGroupState{
						{
							LoadBalancerTargetGroupARN: aws.String("foo"),
						},
					},
				}, nil),
				DetachLoadBalancerTargetGroups: fake.R(nil, nil),
			},
			elbv2Response: fake.ELBv2Outputs{
				DescribeTargetGroups: fake.R(nil, fake.ErrDummy),
			},
			wantError: true,
		},
		{
			name:         "describe-tags-failed",
			targetGroups: []string{"foo"},
			autoscalingOutputs: fake.ASGOutputs{
				AttachLoadBalancerTargetGroups: fake.R(nil, nil),
				DescribeLoadBalancerTargetGroups: fake.R(&autoscaling.DescribeLoadBalancerTargetGroupsOutput{
					LoadBalancerTargetGroups: []types.LoadBalancerTargetGroupState{
						{
							LoadBalancerTargetGroupARN: aws.String("foo"),
						},
					},
				}, nil),
				DetachLoadBalancerTargetGroups: fake.R(nil, nil),
			},
			elbv2Response: fake.ELBv2Outputs{
				DescribeTargetGroups: fake.R(&elbv2.DescribeTargetGroupsOutput{
					TargetGroups: []elbv2types.TargetGroup{
						{
							TargetGroupArn: aws.String("foo"),
						},
					},
				}, nil),
				DescribeTags: fake.R(nil, fake.ErrDummy),
			},
			wantError: true,
		},
		{
			name:         "success-attach",
			targetGroups: []string{"foo"},
			autoscalingOutputs: fake.ASGOutputs{
				AttachLoadBalancerTargetGroups: fake.R(nil, nil),
				DescribeLoadBalancerTargetGroups: fake.R(&autoscaling.DescribeLoadBalancerTargetGroupsOutput{
					LoadBalancerTargetGroups: []types.LoadBalancerTargetGroupState{
						{
							LoadBalancerTargetGroupARN: aws.String("foo"),
						},
					},
				}, nil)},
			elbv2Response: fake.ELBv2Outputs{
				DescribeTargetGroups: fake.R(&elbv2.DescribeTargetGroupsOutput{
					TargetGroups: []elbv2types.TargetGroup{
						{
							TargetGroupArn: aws.String("foo"),
							TargetType:     elbv2types.TargetTypeEnumInstance,
						},
					},
				}, nil),
				DescribeTags: fake.R(&elbv2.DescribeTagsOutput{
					TagDescriptions: []elbv2types.TagDescription{
						{
							ResourceArn: aws.String("foo"),
							Tags: []elbv2types.Tag{
								{
									Key:   aws.String("owner"),
									Value: aws.String("true"),
								},
							},
						},
					},
				}, nil),
			},
			ownerTags: map[string]string{"owner": "true"},
			wantError: false,
		},
		{
			name:         "failed-attach",
			targetGroups: []string{"foo"},
			autoscalingOutputs: fake.ASGOutputs{
				AttachLoadBalancerTargetGroups: fake.R(nil, fake.ErrDummy),
				DescribeLoadBalancerTargetGroups: fake.R(&autoscaling.DescribeLoadBalancerTargetGroupsOutput{
					LoadBalancerTargetGroups: []types.LoadBalancerTargetGroupState{
						{
							LoadBalancerTargetGroupARN: aws.String("foo"),
						},
					},
				}, nil),
			},
			elbv2Response: fake.ELBv2Outputs{
				DescribeTargetGroups: fake.R(&elbv2.DescribeTargetGroupsOutput{
					TargetGroups: []elbv2types.TargetGroup{
						{
							TargetGroupArn: aws.String("foo"),
							TargetType:     elbv2types.TargetTypeEnumInstance,
						},
					},
				}, nil),
				DescribeTags: fake.R(&elbv2.DescribeTagsOutput{
					TagDescriptions: []elbv2types.TagDescription{
						{
							ResourceArn: aws.String("foo"),
							Tags: []elbv2types.Tag{
								{
									Key:   aws.String("owner"),
									Value: aws.String("true"),
								},
							},
						},
					},
				}, nil),
			},
			ownerTags: map[string]string{"owner": "true"},
			wantError: true,
		},
		{
			name:         "detach-obsolete",
			targetGroups: []string{"foo"},
			autoscalingOutputs: fake.ASGOutputs{
				AttachLoadBalancerTargetGroups: fake.R(nil, nil),
				DescribeLoadBalancerTargetGroups: fake.R(&autoscaling.DescribeLoadBalancerTargetGroupsOutput{
					LoadBalancerTargetGroups: []types.LoadBalancerTargetGroupState{
						{
							LoadBalancerTargetGroupARN: aws.String("foo"),
						},
						{
							LoadBalancerTargetGroupARN: aws.String("bar"),
						},
						{
							LoadBalancerTargetGroupARN: aws.String("baz"),
						},
						{
							LoadBalancerTargetGroupARN: aws.String("does-not-exist"),
						},
					},
				}, nil),
				DetachLoadBalancerTargetGroups: fake.R(nil, nil),
			},
			elbv2Response: fake.ELBv2Outputs{
				DescribeTargetGroups: fake.R(&elbv2.DescribeTargetGroupsOutput{
					TargetGroups: []elbv2types.TargetGroup{
						{
							TargetGroupArn: aws.String("foo"),
						},
						{
							TargetGroupArn: aws.String("bar"),
						},
						{
							TargetGroupArn: aws.String("baz"),
						},
					},
				}, nil),
				DescribeTags: fake.R(&elbv2.DescribeTagsOutput{
					TagDescriptions: []elbv2types.TagDescription{
						{
							ResourceArn: aws.String("foo"),
							Tags: []elbv2types.Tag{
								{
									Key:   aws.String("owner"),
									Value: aws.String("true"),
								},
							},
						},
						{
							ResourceArn: aws.String("bar"),
							Tags: []elbv2types.Tag{
								{
									Key:   aws.String("owner"),
									Value: aws.String("true"),
								},
							},
						},
						{
							ResourceArn: aws.String("baz"),
							Tags:        []elbv2types.Tag{},
						},
						{
							ResourceArn: aws.String("does-not-exist"),
							Tags:        []elbv2types.Tag{},
						},
					},
				}, nil),
			},
			ownerTags: map[string]string{"owner": "true"},
			wantError: false,
		},
		{
			name:         "failed-detach",
			targetGroups: []string{"foo"},
			autoscalingOutputs: fake.ASGOutputs{
				AttachLoadBalancerTargetGroups: fake.R(nil, nil),
				DescribeLoadBalancerTargetGroups: fake.R(&autoscaling.DescribeLoadBalancerTargetGroupsOutput{
					LoadBalancerTargetGroups: []types.LoadBalancerTargetGroupState{
						{
							LoadBalancerTargetGroupARN: aws.String("foo"),
						},
						{
							LoadBalancerTargetGroupARN: aws.String("bar"),
						},
					},
				}, nil),
				DetachLoadBalancerTargetGroups: fake.R(nil, fake.ErrDummy),
			},
			elbv2Response: fake.ELBv2Outputs{
				DescribeTargetGroups: fake.R(&elbv2.DescribeTargetGroupsOutput{
					TargetGroups: []elbv2types.TargetGroup{
						{
							TargetGroupArn: aws.String("foo"),
						},
						{
							TargetGroupArn: aws.String("bar"),
						},
					},
				}, nil),
				DescribeTags: fake.R(&elbv2.DescribeTagsOutput{
					TagDescriptions: []elbv2types.TagDescription{
						{
							ResourceArn: aws.String("foo"),
							Tags: []elbv2types.Tag{
								{
									Key:   aws.String("owner"),
									Value: aws.String("true"),
								},
							},
						},
						{
							ResourceArn: aws.String("bar"),
							Tags: []elbv2types.Tag{
								{
									Key:   aws.String("owner"),
									Value: aws.String("true"),
								},
							},
						},
					},
				}, nil),
			},
			ownerTags: map[string]string{"owner": "true"},
			wantError: true,
		},
		{
			name:         "attach ignores nonexistent target groups",
			targetGroups: []string{"foo", "void", "bar", "blank"},
			autoscalingOutputs: fake.ASGOutputs{
				AttachLoadBalancerTargetGroups: fake.R(nil, nil),
				DescribeLoadBalancerTargetGroups: fake.R(&autoscaling.DescribeLoadBalancerTargetGroupsOutput{
					LoadBalancerTargetGroups: []types.LoadBalancerTargetGroupState{
						{
							LoadBalancerTargetGroupARN: aws.String("foo"),
						},
						{
							LoadBalancerTargetGroupARN: aws.String("bar"),
						},
					},
				}, nil)},
			elbv2Response: fake.ELBv2Outputs{
				DescribeTargetGroups: fake.R(&elbv2.DescribeTargetGroupsOutput{
					TargetGroups: []elbv2types.TargetGroup{
						{
							TargetGroupArn: aws.String("foo"),
							TargetType:     elbv2types.TargetTypeEnumInstance,
						},
						{
							TargetGroupArn: aws.String("bar"),
							TargetType:     elbv2types.TargetTypeEnumInstance,
						},
					},
				}, nil),
				DescribeTags: fake.R(&elbv2.DescribeTagsOutput{
					TagDescriptions: []elbv2types.TagDescription{
						{
							ResourceArn: aws.String("foo"),
							Tags:        []elbv2types.Tag{{Key: aws.String("owner"), Value: aws.String("true")}},
						},
						{
							ResourceArn: aws.String("bar"),
							Tags:        []elbv2types.Tag{{Key: aws.String("owner"), Value: aws.String("true")}},
						},
					},
				}, nil),
			},
			autoscalingInputs: fake.ASGInputs{
				AttachLoadBalancerTargetGroups: func(t *testing.T, input *autoscaling.AttachLoadBalancerTargetGroupsInput) {
					assert.Equal(t, aws.String("asg-name"), input.AutoScalingGroupName)
					assert.Equal(t, []string{"foo", "bar"}, input.TargetGroupARNs)
				},
			},
			ownerTags: map[string]string{"owner": "true"},
			wantError: false,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			mockSvc := &fake.ASGClient{Outputs: test.autoscalingOutputs, Inputs: test.autoscalingInputs, T: t}
			mockElbv2Svc := &fake.ELBv2Client{Outputs: test.elbv2Response}
			err := updateTargetGroupsForAutoScalingGroup(context.Background(), mockSvc, mockElbv2Svc, test.targetGroups, "asg-name", test.ownerTags)
			if test.wantError {
				if err == nil {
					t.Error("wanted an error but call seemed to have succeeded")
				}
			} else {
				if err != nil {
					t.Fatal("unexpected error", err)
				}
			}
		})
	}
}

func TestDetach(t *testing.T) {
	for _, test := range []struct {
		name      string
		responses fake.ASGOutputs
		wantError bool
	}{
		{"success-detach", fake.ASGOutputs{DetachLoadBalancerTargetGroups: fake.R(nil, nil)},
			false},
		{"failed-detach", fake.ASGOutputs{DetachLoadBalancerTargetGroups: fake.R(nil, fake.ErrDummy)},
			true},
	} {
		t.Run(fmt.Sprintf("%v", test.name), func(t *testing.T) {
			mockSvc := &fake.ASGClient{Outputs: test.responses}
			err := detachTargetGroupsFromAutoScalingGroup(context.Background(), mockSvc, []string{"foo"}, "bar")
			if test.wantError {
				if err == nil {
					t.Error("wanted an error but call seemed to have succeeded")
				}
			} else {
				if err != nil {
					t.Fatal("unexpected error", err)
				}
			}
		})
	}

}

func TestTestFilterTags(t *testing.T) {
	for _, test := range []struct {
		name         string
		clusterId    string
		customFilter string
		asgTags      map[string]string
		want         bool
	}{
		{
			"success-test-filter-asg",
			"mycluster",
			"tag:kubernetes.io/cluster/mycluster=owned tag-key=k8s.io/role/node tag-key=custom.com/ingress",
			map[string]string{
				"kubernetes.io/cluster/mycluster": "owned",
				"k8s.io/role/node":                "1",
				"custom.com/ingress":              "",
			},
			true,
		},
		{
			"success-test-filter-negative-asg",
			"mycluster",
			"tag:kubernetes.io/cluster/yourcluster=owned tag-key=k8s.io/role/node tag-key=custom.com/ingress",
			map[string]string{
				"kubernetes.io/cluster/mycluster": "owned",
				"k8s.io/role/node":                "1",
				"custom.com/ingress":              "",
			},
			false,
		},
		{
			"success-test-filter-multi-value-asg",
			"mycluster",
			"tag:kubernetes.io/cluster/mycluster=owned tag-key=k8s.io/role/node tag:custom.com/ingress=owned,shared",
			map[string]string{
				"kubernetes.io/cluster/mycluster": "owned",
				"k8s.io/role/node":                "1",
				"custom.com/ingress":              "shared",
			},
			true,
		},
		{
			"test value mismatch",
			"mycluster",
			"tag:kubernetes.io/cluster/mycluster=owned tag:custom.com/ingress=owned",
			map[string]string{
				"kubernetes.io/cluster/mycluster": "owned",
				"custom.com/ingress":              "whatever",
			},
			false,
		},
	} {
		t.Run(fmt.Sprintf("%v", test.name), func(t *testing.T) {
			a := &Adapter{customFilter: test.customFilter}
			filterTags := a.parseAutoscaleFilterTags(test.clusterId)
			got := testFilterTags(filterTags, test.asgTags)

			if !reflect.DeepEqual(test.want, got) {
				t.Errorf("unexpected result. wanted %+v, got %+v", test.want, got)
			}
		})
	}
}

type testChunkProcessor struct {
	failOn int
	chunks [][]string
}

func (p *testChunkProcessor) process(chunk []string) error {
	if p.failOn-1 == len(p.chunks) {
		return fmt.Errorf("failing on %d chunk", p.failOn)
	}
	p.chunks = append(p.chunks, chunk)
	return nil
}

func TestProcessChunked(t *testing.T) {
	const chunkSize = 5

	for _, ti := range []struct {
		name   string
		input  []string
		failOn int
		expect [][]string
		err    bool
	}{
		{
			name:   "empty",
			input:  nil,
			expect: nil,
		},
		{
			name:   "less than chunk size",
			input:  []string{"1", "2", "3", "4"},
			expect: [][]string{{"1", "2", "3", "4"}},
		},
		{
			name:   "equal to chunk size",
			input:  []string{"1", "2", "3", "4", "5"},
			expect: [][]string{{"1", "2", "3", "4", "5"}},
		},
		{
			name:   "greater than chunk size",
			input:  []string{"1", "2", "3", "4", "5", "6"},
			expect: [][]string{{"1", "2", "3", "4", "5"}, {"6"}},
		},
		{
			name:   "fail on first chunk",
			input:  []string{"1", "2", "3", "4", "5", "6"},
			failOn: 1,
			expect: nil,
			err:    true,
		},
		{
			name:   "fail on second chunk",
			input:  []string{"1", "2", "3", "4", "5", "6"},
			failOn: 2,
			expect: [][]string{{"1", "2", "3", "4", "5"}},
			err:    true,
		},
	} {
		t.Run(ti.name, func(t *testing.T) {
			cp := testChunkProcessor{failOn: ti.failOn}
			err := processChunked(ti.input, chunkSize, cp.process)
			if ti.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, ti.expect, cp.chunks)
		})
	}
}

func Test_categorizeTargetTypeInstance(t *testing.T) {
	for _, test := range []struct {
		name         string
		targetGroups []elbv2types.TargetGroup
	}{
		{
			name: "one from any type",
			targetGroups: []elbv2types.TargetGroup{
				{
					TargetGroupArn: aws.String("instancy"),
					TargetType:     elbv2types.TargetTypeEnumInstance,
				},
				{
					TargetGroupArn: aws.String("albly"),
					TargetType:     elbv2types.TargetTypeEnumAlb,
				},
				{
					TargetGroupArn: aws.String("ipvy"),
					TargetType:     elbv2types.TargetTypeEnumIp,
				},
				{
					TargetGroupArn: aws.String("lambada"),
					TargetType:     elbv2types.TargetTypeEnumLambda,
				},
			},
		},
		{
			name: "one type many target groups",
			targetGroups: []elbv2types.TargetGroup{
				{
					TargetGroupArn: aws.String("instancy"),
					TargetType:     elbv2types.TargetTypeEnumInstance,
				},
				{
					TargetGroupArn: aws.String("foo"),
					TargetType:     elbv2types.TargetTypeEnumInstance,
				},
				{
					TargetGroupArn: aws.String("void"),
					TargetType:     elbv2types.TargetTypeEnumInstance,
				},
				{
					TargetGroupArn: aws.String("bar"),
					TargetType:     elbv2types.TargetTypeEnumInstance,
				},
				{
					TargetGroupArn: aws.String("blank"),
					TargetType:     elbv2types.TargetTypeEnumInstance,
				},
			},
		},
		{
			name: "several types many target groups",
			targetGroups: []elbv2types.TargetGroup{
				{
					TargetGroupArn: aws.String("instancy"),
					TargetType:     elbv2types.TargetTypeEnumInstance,
				},
				{
					TargetGroupArn: aws.String("foo"),
					TargetType:     elbv2types.TargetTypeEnumInstance,
				},
				{
					TargetGroupArn: aws.String("void"),
					TargetType:     elbv2types.TargetTypeEnumInstance,
				},
				{
					TargetGroupArn: aws.String("bar"),
					TargetType:     elbv2types.TargetTypeEnumInstance,
				},
				{
					TargetGroupArn: aws.String("blank"),
					TargetType:     elbv2types.TargetTypeEnumInstance,
				},
				{
					TargetGroupArn: aws.String("albly"),
					TargetType:     elbv2types.TargetTypeEnumAlb,
				},
				{
					TargetGroupArn: aws.String("alblily"),
					TargetType:     elbv2types.TargetTypeEnumAlb,
				},
				{
					TargetGroupArn: aws.String("ipvy"),
					TargetType:     elbv2types.TargetTypeEnumIp,
				},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			allARNs := []string{}
			want := map[elbv2types.TargetTypeEnum][]string{}
			for _, tg := range test.targetGroups {
				allARNs = append(allARNs, *tg.TargetGroupArn)
				if _, ok := want[tg.TargetType]; !ok {
					want[tg.TargetType] = []string{*tg.TargetGroupArn}
				} else {
					want[tg.TargetType] = append(want[tg.TargetType], *tg.TargetGroupArn)
				}
			}
			mockElbv2Svc := &fake.ELBv2Client{Outputs: fake.ELBv2Outputs{DescribeTargetGroups: fake.R(&elbv2.DescribeTargetGroupsOutput{TargetGroups: test.targetGroups}, nil)}}
			got, err := categorizeTargetTypeInstance(context.Background(), mockElbv2Svc, allARNs)
			assert.NoError(t, err)
			assert.Equal(t, want, got)
		})
	}
}
