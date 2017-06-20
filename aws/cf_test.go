package aws

import (
	"reflect"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
)

func TestCreatingStack(t *testing.T) {
	for _, ti := range []struct {
		name         string
		givenSpec    createStackSpec
		givenOutputs cfMockOutputs
		want         string
		wantErr      bool
	}{
		{
			"successful-call",
			createStackSpec{name: "foo", securityGroupID: "bar", vpcID: "baz"},
			cfMockOutputs{createStack: R(mockCSOutput("fake-stack-id"), nil)},
			"fake-stack-id",
			false,
		},
		{
			"successful-call",
			createStackSpec{name: "foo", securityGroupID: "bar", vpcID: "baz"},
			cfMockOutputs{createStack: R(mockCSOutput("fake-stack-id"), nil)},
			"fake-stack-id",
			false,
		},
		{
			"fail-call",
			createStackSpec{name: "foo", securityGroupID: "bar", vpcID: "baz"},
			cfMockOutputs{createStack: R(nil, dummyErr)},
			"fake-stack-id",
			true,
		},
	} {
		t.Run(ti.name, func(t *testing.T) {
			c := &mockCloudFormationClient{outputs: ti.givenOutputs}
			got, err := createStack(c, &ti.givenSpec)
			if ti.wantErr {
				if !ti.wantErr {
					t.Error("unexpected error", err)
				}
			} else {
				if ti.want != got {
					t.Errorf("unexpected result. wanted %+v, got %+v", ti.want, got)
				}
			}
		})
	}
}

func TestStackReadiness(t *testing.T) {
	for _, ti := range []struct {
		given string
		want  bool
	}{
		{cloudformation.StackStatusCreateComplete, true},
		{cloudformation.StackStatusUpdateComplete, true},
		{cloudformation.StackStatusCreateInProgress, false},
		{cloudformation.StackStatusDeleteComplete, false},
		{cloudformation.StackStatusDeleteFailed, false},
		{cloudformation.StackStatusDeleteInProgress, false},
		{cloudformation.StackStatusReviewInProgress, false},
		{cloudformation.StackStatusRollbackComplete, false},
		{cloudformation.StackStatusRollbackFailed, false},
		{cloudformation.StackStatusRollbackInProgress, false},
		{cloudformation.StackStatusUpdateCompleteCleanupInProgress, false},
		{cloudformation.StackStatusUpdateRollbackCompleteCleanupInProgress, false},
		{"dummy-status", false},
	} {
		t.Run(ti.given, func(t *testing.T) {
			got := isComplete(aws.String(ti.given))
			if ti.want != got {
				t.Errorf("unexpected result. wanted %+v, got %+v", ti.want, got)
			}
		})
	}

}

func TestManagementAssertion(t *testing.T) {
	for _, ti := range []struct {
		name  string
		given []*cloudformation.Tag
		want  bool
	}{
		{"managed", []*cloudformation.Tag{
			cfTag(kubernetesCreatorTag, kubernetesCreatorValue),
			cfTag(clusterIDTag, "test-cluster"),
			cfTag("foo", "bar"),
		}, true},
		{"missing-cluster-tag", []*cloudformation.Tag{
			cfTag(kubernetesCreatorTag, kubernetesCreatorValue),
		}, false},
		{"missing-kube-mgmt-tag", []*cloudformation.Tag{
			cfTag(clusterIDTag, "test-cluster"),
		}, false},
		{"missing-all-mgmt-tag", []*cloudformation.Tag{
			cfTag("foo", "bar"),
		}, false},
		{"mismatch-cluster-tag", []*cloudformation.Tag{
			cfTag(kubernetesCreatorTag, kubernetesCreatorValue),
			cfTag(clusterIDTag, "other-cluster"),
			cfTag("foo", "bar"),
		}, false},
	} {
		t.Run(ti.name, func(t *testing.T) {
			got := isManagedStack(ti.given, "test-cluster")
			if ti.want != got {
				t.Errorf("unexpected result. wanted %+v, got %+v", ti.want, got)
			}
		})
	}
}

func TestTagConversion(t *testing.T) {
	for _, ti := range []struct {
		name  string
		given []*cloudformation.Tag
		want  map[string]string
	}{
		{"default", []*cloudformation.Tag{cfTag("foo", "bar")}, map[string]string{"foo": "bar"}},
		{"empty-input", []*cloudformation.Tag{}, map[string]string{}},
		{"nil-input", nil, map[string]string{}},
	} {
		t.Run(ti.name, func(t *testing.T) {
			got := convertCloudFormationTags(ti.given)
			if !reflect.DeepEqual(ti.want, got) {
				t.Errorf("unexpected result. wanted %+v, got %+v", ti.want, got)
			}
		})
	}

}

func TestFindingManagedStacks(t *testing.T) {
	for _, ti := range []struct {
		name    string
		given   cfMockOutputs
		want    []*Stack
		wantErr bool
	}{
		{
			"successful-call",
			cfMockOutputs{
				describeStackPages: R(nil, nil),
				describeStacks: R(&cloudformation.DescribeStacksOutput{
					Stacks: []*cloudformation.Stack{
						{
							StackName: aws.String("managed-stack-not-ready"),
							Tags: []*cloudformation.Tag{
								cfTag(kubernetesCreatorTag, kubernetesCreatorValue),
								cfTag(clusterIDTag, "test-cluster"),
								cfTag(certificateARNTag, "cert-arn"),
							},
							Outputs: []*cloudformation.Output{
								{OutputKey: aws.String(outputLoadBalancerDNSName), OutputValue: aws.String("example-notready.com")},
								{OutputKey: aws.String(outputTargetGroupARN), OutputValue: aws.String("tg-arn")},
							},
						},
						{
							StackName:   aws.String("managed-stack"),
							StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
							Tags: []*cloudformation.Tag{
								cfTag(kubernetesCreatorTag, kubernetesCreatorValue),
								cfTag(clusterIDTag, "test-cluster"),
								cfTag(certificateARNTag, "cert-arn"),
							},
							Outputs: []*cloudformation.Output{
								{OutputKey: aws.String(outputLoadBalancerDNSName), OutputValue: aws.String("example.com")},
								{OutputKey: aws.String(outputTargetGroupARN), OutputValue: aws.String("tg-arn")},
							},
						},
						{
							StackName: aws.String("managed-stack-not-ready"),
							Tags: []*cloudformation.Tag{
								cfTag(kubernetesCreatorTag, kubernetesCreatorValue),
								cfTag(clusterIDTag, "test-cluster"),
							},
						},
						{
							StackName: aws.String("unmanaged-stack"),
							Tags: []*cloudformation.Tag{
								cfTag(clusterIDTag, "test-cluster"),
							},
						},
						{
							StackName: aws.String("another-unmanaged-stack"),
							Tags: []*cloudformation.Tag{
								cfTag(kubernetesCreatorTag, kubernetesCreatorValue),
							},
						},
						{
							StackName: aws.String("belongs-to-other-cluster"),
							Tags: []*cloudformation.Tag{
								cfTag(kubernetesCreatorTag, kubernetesCreatorValue),
								cfTag(clusterIDTag, "other-cluster"),
							},
						},
					},
				}, nil),
			},
			[]*Stack{
				{
					name:           "managed-stack",
					dnsName:        "example.com",
					certificateARN: "cert-arn",
					targetGroupARN: "tg-arn",
					tags: map[string]string{
						kubernetesCreatorTag: kubernetesCreatorValue,
						clusterIDTag:         "test-cluster",
						certificateARNTag:    "cert-arn",
					},
				},
			},
			false,
		},
		{
			"no-ready-stacks",
			cfMockOutputs{
				describeStackPages: R(nil, nil),
				describeStacks: R(&cloudformation.DescribeStacksOutput{
					Stacks: []*cloudformation.Stack{
						{
							StackName:   aws.String("managed-stack-not-ready"),
							StackStatus: aws.String(cloudformation.StackStatusReviewInProgress),
							Tags: []*cloudformation.Tag{
								cfTag(kubernetesCreatorTag, kubernetesCreatorValue),
								cfTag(clusterIDTag, "test-cluster"),
							},
							Outputs: []*cloudformation.Output{
								{OutputKey: aws.String(outputLoadBalancerDNSName), OutputValue: aws.String("example-notready.com")},
								{OutputKey: aws.String(outputTargetGroupARN), OutputValue: aws.String("tg-arn")},
							},
						},
						{
							StackName:   aws.String("managed-stack"),
							StackStatus: aws.String(cloudformation.StackStatusRollbackComplete),
							Tags: []*cloudformation.Tag{
								cfTag(kubernetesCreatorTag, kubernetesCreatorValue),
								cfTag(clusterIDTag, "test-cluster"),
							},
							Outputs: []*cloudformation.Output{
								{OutputKey: aws.String(outputLoadBalancerDNSName), OutputValue: aws.String("example.com")},
								{OutputKey: aws.String(outputTargetGroupARN), OutputValue: aws.String("tg-arn")},
							},
						},
					},
				}, nil),
			},
			[]*Stack{},
			true,
		},
		{
			"failed-paging",
			cfMockOutputs{
				describeStackPages: R(nil, dummyErr),
				describeStacks:     R(&cloudformation.DescribeStacksOutput{}, nil),
			},
			nil,
			true,
		},
		{
			"failed-describe-page",
			cfMockOutputs{
				describeStacks: R(nil, dummyErr),
			},
			nil,
			true,
		},
	} {
		t.Run(ti.name, func(t *testing.T) {
			c := &mockCloudFormationClient{outputs: ti.given}
			got, err := findManagedStacks(c, "test-cluster")
			if err != nil {
				if !ti.wantErr {
					t.Error("unexpected error", err)
				}
			} else {
				if !reflect.DeepEqual(ti.want, got) {
					t.Errorf("unexpected result. wanted %+v, got %+v", ti.want, got)
				}
			}

			s, err := getStack(c, "dontcare")
			if err != nil {
				if !ti.wantErr {
					t.Error("unexpected error", err)
				}
			} else {
				if !reflect.DeepEqual(ti.want[0], s) {
					t.Errorf("unexpected result. wanted %+v, got %+v", ti.want[0], got)
				}
			}

		})
	}
}

func TestIsDeleteInProgress(t *testing.T) {
	for _, ti := range []struct {
		msg   string
		given *Stack
		want  bool
	}{
		{
			"DeleteInProgress",
			&Stack{tags: map[string]string{deleteScheduled: time.Now().Add(1 * time.Minute).Format(time.RFC3339)}},
			true,
		},
		{
			"EmptyStack",
			&Stack{},
			false,
		},
		{
			"StackNil",
			nil,
			false,
		},
	} {
		t.Run(ti.msg, func(t *testing.T) {
			got := ti.given.IsDeleteInProgress()
			if ti.want != got {
				t.Errorf("unexpected result. wanted %+v, got %+v", ti.want, got)
			}
		})
	}

}

func TestShouldDelete(t *testing.T) {
	for _, ti := range []struct {
		msg   string
		given *Stack
		want  bool
	}{
		{
			"DeleteInProgress",
			&Stack{tags: map[string]string{deleteScheduled: time.Now().Add(1 * time.Minute).Format(time.RFC3339)}},
			false,
		},
		{
			"DeleteInProgressSecond",
			&Stack{tags: map[string]string{deleteScheduled: time.Now().Add(1 * time.Second).Format(time.RFC3339)}},
			false,
		},
		{
			"ShouldDelete",
			&Stack{tags: map[string]string{deleteScheduled: time.Now().Add(-1 * time.Second).Format(time.RFC3339)}},
			true,
		},
		{
			"ShouldDeleteMinute",
			&Stack{tags: map[string]string{deleteScheduled: time.Now().Add(-1 * time.Minute).Format(time.RFC3339)}},
			true,
		}, {
			"EmptyStack",
			&Stack{},
			false,
		},
		{
			"StackNil",
			nil,
			false,
		},
	} {
		t.Run(ti.msg, func(t *testing.T) {
			got := ti.given.ShouldDelete()
			if ti.want != got {
				t.Errorf("unexpected result for %s. wanted %+v, got %+v", ti.msg, ti.want, got)
			}
		})
	}

}

func TestDeleteTime(t *testing.T) {
	now := time.Now()
	for _, ti := range []struct {
		msg   string
		given *Stack
		want  *time.Time
	}{
		{
			"GetCorrectTime",
			&Stack{tags: map[string]string{deleteScheduled: now.Format(time.RFC3339Nano)}},
			&now,
		},
		{
			"EmptyStack",
			&Stack{},
			nil,
		},
		{
			"StackNil",
			nil,
			nil,
		},
	} {
		t.Run(ti.msg, func(t *testing.T) {
			got := ti.given.deleteTime()
			if ti.want != nil {
				if !ti.want.Equal(*got) {
					t.Errorf("unexpected result for non nil %s. wanted %+v, got %+v", ti.msg, ti.want, got)
				}
			} else if ti.want != got {
				t.Errorf("unexpected result for %s. wanted %+v, got %+v", ti.msg, ti.want, got)
			}
		})
	}

}
