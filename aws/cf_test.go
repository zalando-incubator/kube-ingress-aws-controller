package aws

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/stretchr/testify/assert"

	"github.com/zalando-incubator/kube-ingress-aws-controller/aws/fake"
)

func TestCreatingStack(t *testing.T) {
	for _, ti := range []struct {
		name         string
		givenSpec    stackSpec
		givenOutputs fake.CFOutputs
		want         string
		wantErr      bool
	}{
		{
			"successful-call",
			stackSpec{
				name:            "foo",
				securityGroupID: "bar",
				vpcID:           "baz",
				certificateARNs: map[string]time.Time{
					"arn-default": {},
					"arn-second":  {},
				},
			},
			fake.CFOutputs{CreateStack: fake.R(fake.MockCSOutput("fake-stack-id"), nil)},
			"fake-stack-id",
			false,
		},
		{
			"successful-call",
			stackSpec{name: "foo", securityGroupID: "bar", vpcID: "baz"},
			fake.CFOutputs{CreateStack: fake.R(fake.MockCSOutput("fake-stack-id"), nil)},
			"fake-stack-id",
			false,
		},
		{
			"fail-call",
			stackSpec{name: "foo", securityGroupID: "bar", vpcID: "baz"},
			fake.CFOutputs{CreateStack: fake.R(nil, fake.ErrDummy)},
			"fake-stack-id",
			true,
		},
		{
			"stack with WAF association",
			stackSpec{
				name:            "foo",
				securityGroupID: "bar",
				vpcID:           "baz",
				wafWebAclId:     "foo-bar-baz",
			},
			fake.CFOutputs{CreateStack: fake.R(fake.MockCSOutput("fake-stack-id"), nil)},
			"fake-stack-id",
			false,
		},
		{
			"stack with ALB http port",
			stackSpec{
				name:             "foo",
				securityGroupID:  "bar",
				vpcID:            "baz",
				loadbalancerType: LoadBalancerTypeApplication,
				httpTargetPort:   7777,
			},
			fake.CFOutputs{CreateStack: fake.R(fake.MockCSOutput("fake-stack-id"), nil)},
			"fake-stack-id",
			false,
		},
		{
			"stack with NLB http port",
			stackSpec{
				name:             "foo",
				securityGroupID:  "bar",
				vpcID:            "baz",
				loadbalancerType: LoadBalancerTypeNetwork,
				httpTargetPort:   8888,
			},
			fake.CFOutputs{CreateStack: fake.R(fake.MockCSOutput("fake-stack-id"), nil)},
			"fake-stack-id",
			false,
		},
	} {
		t.Run(ti.name, func(t *testing.T) {
			c := &fake.CFClient{Outputs: ti.givenOutputs}
			got, err := createStack(context.Background(), c, &ti.givenSpec)
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

func TestUpdatingStack(t *testing.T) {
	for _, ti := range []struct {
		name         string
		givenSpec    stackSpec
		givenOutputs fake.CFOutputs
		want         string
		wantErr      bool
	}{
		{
			"successful-call",
			stackSpec{
				name:            "foo",
				securityGroupID: "bar",
				vpcID:           "baz",
				certificateARNs: map[string]time.Time{
					"arn-default": {},
					"arn-second":  {},
				},
			},
			fake.CFOutputs{UpdateStack: fake.R(fake.MockUSOutput("fake-stack-id"), nil)},
			"fake-stack-id",
			false,
		},
		{
			"successful-call",
			stackSpec{name: "foo", securityGroupID: "bar", vpcID: "baz"},
			fake.CFOutputs{UpdateStack: fake.R(fake.MockUSOutput("fake-stack-id"), nil)},
			"fake-stack-id",
			false,
		},
		{
			"fail-call",
			stackSpec{name: "foo", securityGroupID: "bar", vpcID: "baz"},
			fake.CFOutputs{UpdateStack: fake.R(nil, fake.ErrDummy)},
			"fake-stack-id",
			true,
		},
		{
			"stack with WAF association",
			stackSpec{
				name:            "foo",
				securityGroupID: "bar",
				vpcID:           "baz",
				wafWebAclId:     "foo-bar-baz",
			},
			fake.CFOutputs{UpdateStack: fake.R(fake.MockUSOutput("fake-stack-id"), nil)},
			"fake-stack-id",
			false,
		},
		{
			"stack with ALB http port",
			stackSpec{
				name:             "foo",
				securityGroupID:  "bar",
				vpcID:            "baz",
				loadbalancerType: LoadBalancerTypeApplication,
				httpTargetPort:   7777,
			},
			fake.CFOutputs{UpdateStack: fake.R(fake.MockUSOutput("fake-stack-id"), nil)},
			"fake-stack-id",
			false,
		},
		{
			"stack with NLB http port",
			stackSpec{
				name:             "foo",
				securityGroupID:  "bar",
				vpcID:            "baz",
				loadbalancerType: LoadBalancerTypeNetwork,
				httpTargetPort:   8888,
			},
			fake.CFOutputs{UpdateStack: fake.R(fake.MockUSOutput("fake-stack-id"), nil)},
			"fake-stack-id",
			false,
		},
	} {
		t.Run(ti.name, func(t *testing.T) {
			c := &fake.CFClient{Outputs: ti.givenOutputs}
			got, err := updateStack(context.Background(), c, &ti.givenSpec)
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

func TestDeleteStack(t *testing.T) {
	for _, ti := range []struct {
		msg          string
		givenSpec    stackSpec
		givenOutputs fake.CFOutputs
		wantErr      bool
	}{
		{
			"delete-existing-stack",
			stackSpec{name: "existing-stack-id"},
			fake.CFOutputs{
				DeleteStack:                 fake.R(fake.MockDeleteStackOutput("existing-stack-id"), nil),
				UpdateTerminationProtection: fake.R(nil, nil),
			},
			false,
		},
		{
			"delete-non-existing-stack",
			stackSpec{name: "non-existing-stack-id"},
			fake.CFOutputs{
				DeleteStack:                 fake.R(fake.MockDeleteStackOutput("existing-stack-id"), nil),
				UpdateTerminationProtection: fake.R(nil, nil),
			},
			false,
		},
	} {
		t.Run(ti.msg, func(t *testing.T) {
			c := &fake.CFClient{Outputs: ti.givenOutputs}
			err := deleteStack(context.Background(), c, ti.givenSpec.name)
			haveErr := err != nil
			if haveErr != ti.wantErr {
				t.Errorf("unexpected result from %s. wanted error %v, got err: %+v", ti.msg, ti.wantErr, err)
			}
		})
	}
}

func TestIsComplete(t *testing.T) {
	for _, ti := range []struct {
		given types.StackStatus
		want  bool
	}{
		{types.StackStatusCreateComplete, true},
		{types.StackStatusUpdateComplete, true},
		{types.StackStatusCreateInProgress, false},
		{types.StackStatusDeleteComplete, false},
		{types.StackStatusDeleteFailed, false},
		{types.StackStatusDeleteInProgress, false},
		{types.StackStatusReviewInProgress, false},
		{types.StackStatusRollbackComplete, true},
		{types.StackStatusRollbackFailed, false},
		{types.StackStatusRollbackInProgress, false},
		{types.StackStatusUpdateCompleteCleanupInProgress, false},
		{types.StackStatusUpdateRollbackCompleteCleanupInProgress, false},
		{"dummy-status", false},
	} {
		t.Run(string(ti.given), func(t *testing.T) {
			stack := &Stack{status: ti.given}
			got := stack.IsComplete()
			if ti.want != got {
				t.Errorf("unexpected result. wanted %+v, got %+v", ti.want, got)
			}
		})
	}

}

func TestErr(t *testing.T) {
	const NONE = ""
	for _, ti := range []struct {
		stack         *Stack
		expectedError string
	}{
		{stack: nil, expectedError: NONE},
		{stack: &Stack{status: types.StackStatusCreateInProgress}, expectedError: NONE},
		{stack: &Stack{status: types.StackStatusCreateComplete}, expectedError: NONE},
		{stack: &Stack{status: types.StackStatusUpdateInProgress}, expectedError: NONE},
		{stack: &Stack{status: types.StackStatusUpdateComplete}, expectedError: NONE},
		{stack: &Stack{status: types.StackStatusUpdateCompleteCleanupInProgress}, expectedError: NONE},
		{stack: &Stack{status: types.StackStatusDeleteInProgress}, expectedError: NONE},
		{stack: &Stack{status: types.StackStatusDeleteComplete}, expectedError: NONE},
		//
		{stack: &Stack{status: types.StackStatusUpdateRollbackComplete}, expectedError: "unexpected status UPDATE_ROLLBACK_COMPLETE"},
		{
			stack: &Stack{
				status:       types.StackStatusUpdateRollbackInProgress,
				statusReason: "Parameter validation failed: parameter value sg-xxx for parameter name LoadBalancerSecurityGroupParameter does not exist",
			},
			expectedError: "unexpected status UPDATE_ROLLBACK_IN_PROGRESS: Parameter validation failed: parameter value sg-xxx for parameter name LoadBalancerSecurityGroupParameter does not exist",
		},
	} {
		testName := "nil stack"
		if ti.stack != nil {
			testName = string(ti.stack.status)
		}
		t.Run(testName, func(t *testing.T) {
			err := ti.stack.Err()
			if ti.expectedError == NONE {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, ti.expectedError)
			}
		})
	}
}

func TestManagementAssertion(t *testing.T) {
	for _, ti := range []struct {
		name  string
		given []types.Tag
		want  bool
	}{
		{"managed", []types.Tag{
			cfTag(kubernetesCreatorTag, DefaultControllerID),
			cfTag(clusterIDTagPrefix+"test-cluster", resourceLifecycleOwned),
			cfTag("foo", "bar"),
		}, true},
		{"missing-cluster-tag", []types.Tag{
			cfTag(kubernetesCreatorTag, DefaultControllerID),
		}, false},
		{"missing-kube-mgmt-tag", []types.Tag{
			cfTag(clusterIDTagPrefix+"test-cluster", resourceLifecycleOwned),
		}, false},
		{"missing-all-mgmt-tag", []types.Tag{
			cfTag("foo", "bar"),
		}, false},
		{"mismatch-cluster-tag", []types.Tag{
			cfTag(kubernetesCreatorTag, DefaultControllerID),
			cfTag(clusterIDTagPrefix+"other-cluster", resourceLifecycleOwned),
			cfTag("foo", "bar"),
		}, false},
		{"mismatch-controller-id", []types.Tag{
			cfTag(kubernetesCreatorTag, "the-other-one"),
			cfTag(clusterIDTagPrefix+"other-cluster", resourceLifecycleOwned),
			cfTag("foo", "bar"),
		}, false},
	} {
		t.Run(ti.name, func(t *testing.T) {
			got := isManagedStack(ti.given, "test-cluster", DefaultControllerID)
			if ti.want != got {
				t.Errorf("unexpected result. wanted %+v, got %+v", ti.want, got)
			}
		})
	}
}

func TestTagConversion(t *testing.T) {
	for _, ti := range []struct {
		name  string
		given []types.Tag
		want  map[string]string
	}{
		{"default", []types.Tag{cfTag("foo", "bar")}, map[string]string{"foo": "bar"}},
		{"empty-input", []types.Tag{}, map[string]string{}},
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

func TestConvertStackParameters(t *testing.T) {
	for _, ti := range []struct {
		name  string
		given []types.Parameter
		want  map[string]string
	}{
		{"default", []types.Parameter{
			{
				ParameterKey:   aws.String("foo"),
				ParameterValue: aws.String("bar"),
			},
		}, map[string]string{"foo": "bar"}},
		{"empty-input", []types.Parameter{}, map[string]string{}},
		{"nil-input", nil, map[string]string{}},
	} {
		t.Run(ti.name, func(t *testing.T) {
			got := convertStackParameters(ti.given)
			if !reflect.DeepEqual(ti.want, got) {
				t.Errorf("unexpected result. wanted %+v, got %+v", ti.want, got)
			}
		})
	}

}

func TestFindManagedStacks(t *testing.T) {
	for _, ti := range []struct {
		name    string
		given   fake.CFOutputs
		want    []*Stack
		wantErr bool
	}{
		{
			name: "successful-call",
			given: fake.CFOutputs{
				DescribeStacks: fake.R(&cloudformation.DescribeStacksOutput{
					Stacks: []types.Stack{
						{
							StackName:   aws.String("managed-stack-not-ready"),
							StackStatus: types.StackStatusUpdateInProgress,
							Tags: []types.Tag{
								cfTag(kubernetesCreatorTag, DefaultControllerID),
								cfTag(clusterIDTagPrefix+"test-cluster", resourceLifecycleOwned),
								cfTag(certificateARNTagPrefix+"cert-arn", time.Time{}.Format(time.RFC3339)),
							},
							Outputs: []types.Output{
								{OutputKey: aws.String(outputLoadBalancerDNSName), OutputValue: aws.String("example-notready.com")},
								{OutputKey: aws.String(outputTargetGroupARN), OutputValue: aws.String("tg-arn")},
							},
						},
						{
							StackName:   aws.String("managed-stack"),
							StackStatus: types.StackStatusCreateComplete,
							Tags: []types.Tag{
								cfTag(kubernetesCreatorTag, DefaultControllerID),
								cfTag(clusterIDTagPrefix+"test-cluster", resourceLifecycleOwned),
								cfTag(certificateARNTagPrefix+"cert-arn", time.Time{}.Format(time.RFC3339)),
							},
							Outputs: []types.Output{
								{OutputKey: aws.String(outputLoadBalancerDNSName), OutputValue: aws.String("example.com")},
								{OutputKey: aws.String(outputTargetGroupARN), OutputValue: aws.String("tg-arn")},
							},
						},
						{
							StackName:   aws.String("managed-stack-http-arn"),
							StackStatus: types.StackStatusCreateComplete,
							Tags: []types.Tag{
								cfTag(kubernetesCreatorTag, DefaultControllerID),
								cfTag(clusterIDTagPrefix+"test-cluster", resourceLifecycleOwned),
								cfTag(certificateARNTagPrefix+"cert-arn", time.Time{}.Format(time.RFC3339)),
							},
							Outputs: []types.Output{
								{OutputKey: aws.String(outputLoadBalancerDNSName), OutputValue: aws.String("example.com")},
								{OutputKey: aws.String(outputTargetGroupARN), OutputValue: aws.String("tg-arn")},
								{OutputKey: aws.String(outputHTTPTargetGroupARN), OutputValue: aws.String("http-tg-arn")},
							},
						},
						{
							StackName:   aws.String("managed-stack-not-ready"),
							StackStatus: types.StackStatusUpdateInProgress,
							Tags: []types.Tag{
								cfTag(kubernetesCreatorTag, DefaultControllerID),
								cfTag(clusterIDTagPrefix+"test-cluster", resourceLifecycleOwned),
							},
						},
						{
							StackName: aws.String("unmanaged-stack"),
							Tags: []types.Tag{
								cfTag(clusterIDTagPrefix+"test-cluster", resourceLifecycleOwned),
							},
						},
						{
							StackName: aws.String("another-unmanaged-stack"),
							Tags: []types.Tag{
								cfTag(kubernetesCreatorTag, DefaultControllerID),
							},
						},
						{
							StackName: aws.String("belongs-to-other-cluster"),
							Tags: []types.Tag{
								cfTag(kubernetesCreatorTag, DefaultControllerID),
								cfTag(clusterIDTagPrefix+"other-cluster", resourceLifecycleOwned),
							},
						},
					},
				}, nil),
			},
			want: []*Stack{
				{
					Name:    "managed-stack-not-ready",
					DNSName: "example-notready.com",
					CertificateARNs: map[string]time.Time{
						"cert-arn": {},
					},
					TargetGroupARNs: []string{"tg-arn"},
					tags: map[string]string{
						kubernetesCreatorTag:                 DefaultControllerID,
						clusterIDTagPrefix + "test-cluster":  resourceLifecycleOwned,
						certificateARNTagPrefix + "cert-arn": time.Time{}.Format(time.RFC3339),
					},
					status: types.StackStatusUpdateInProgress,
					HTTP2:  true,
				},
				{
					Name:    "managed-stack",
					DNSName: "example.com",
					CertificateARNs: map[string]time.Time{
						"cert-arn": {},
					},
					TargetGroupARNs: []string{"tg-arn"},
					tags: map[string]string{
						kubernetesCreatorTag:                 DefaultControllerID,
						clusterIDTagPrefix + "test-cluster":  resourceLifecycleOwned,
						certificateARNTagPrefix + "cert-arn": time.Time{}.Format(time.RFC3339),
					},
					status: types.StackStatusCreateComplete,
					HTTP2:  true,
				},
				{
					Name:    "managed-stack-http-arn",
					DNSName: "example.com",
					CertificateARNs: map[string]time.Time{
						"cert-arn": {},
					},
					TargetGroupARNs: []string{"tg-arn", "http-tg-arn"},
					tags: map[string]string{
						kubernetesCreatorTag:                 DefaultControllerID,
						clusterIDTagPrefix + "test-cluster":  resourceLifecycleOwned,
						certificateARNTagPrefix + "cert-arn": time.Time{}.Format(time.RFC3339),
					},
					status: types.StackStatusCreateComplete,
					HTTP2:  true,
				},
				{
					Name:            "managed-stack-not-ready",
					CertificateARNs: map[string]time.Time{},
					tags: map[string]string{
						kubernetesCreatorTag:                DefaultControllerID,
						clusterIDTagPrefix + "test-cluster": resourceLifecycleOwned,
					},
					status: types.StackStatusUpdateInProgress,
					HTTP2:  true,
				},
			},
			wantErr: false,
		},
		{
			name: "successfull-call-with-rollback-status",
			given: fake.CFOutputs{
				DescribeStacks: fake.R(&cloudformation.DescribeStacksOutput{
					Stacks: []types.Stack{
						{
							StackName:   aws.String("managed-stack-rolling-back"),
							StackStatus: types.StackStatusRollbackInProgress,
							Tags: []types.Tag{
								cfTag(kubernetesCreatorTag, DefaultControllerID),
								cfTag(clusterIDTagPrefix+"test-cluster", resourceLifecycleOwned),
								cfTag(certificateARNTagPrefix+"cert-arn", time.Time{}.Format(time.RFC3339)),
							},
							Outputs: []types.Output{},
						},
					},
				}, nil),
			},
			want: []*Stack{
				{
					Name: "managed-stack-rolling-back",
					CertificateARNs: map[string]time.Time{
						"cert-arn": {},
					},
					tags: map[string]string{
						kubernetesCreatorTag:                 DefaultControllerID,
						clusterIDTagPrefix + "test-cluster":  resourceLifecycleOwned,
						certificateARNTagPrefix + "cert-arn": time.Time{}.Format(time.RFC3339),
					},
					status: types.StackStatusRollbackInProgress,
					HTTP2:  true,
				},
			},
		},
		{
			name: "no-ready-stacks",
			given: fake.CFOutputs{
				DescribeStacks: fake.R(&cloudformation.DescribeStacksOutput{
					Stacks: []types.Stack{
						{
							StackName:   aws.String("managed-stack-not-ready"),
							StackStatus: types.StackStatusReviewInProgress,
							Tags: []types.Tag{
								cfTag(kubernetesCreatorTag, DefaultControllerID),
								cfTag(clusterIDTagPrefix+"test-cluster", resourceLifecycleOwned),
							},
							Outputs: []types.Output{
								{OutputKey: aws.String(outputLoadBalancerDNSName), OutputValue: aws.String("example-notready.com")},
								{OutputKey: aws.String(outputTargetGroupARN), OutputValue: aws.String("tg-arn")},
							},
						},
						{
							StackName:   aws.String("managed-stack"),
							StackStatus: types.StackStatusRollbackComplete,
							Tags: []types.Tag{
								cfTag(kubernetesCreatorTag, DefaultControllerID),
								cfTag(clusterIDTagPrefix+"test-cluster", resourceLifecycleOwned),
							},
							Outputs: []types.Output{
								{OutputKey: aws.String(outputLoadBalancerDNSName), OutputValue: aws.String("example.com")},
								{OutputKey: aws.String(outputTargetGroupARN), OutputValue: aws.String("tg-arn")},
							},
						},
					},
				}, nil),
			},
			want: []*Stack{
				{
					Name:            "managed-stack-not-ready",
					DNSName:         "example-notready.com",
					TargetGroupARNs: []string{"tg-arn"},
					CertificateARNs: map[string]time.Time{},
					tags: map[string]string{
						kubernetesCreatorTag:                DefaultControllerID,
						clusterIDTagPrefix + "test-cluster": resourceLifecycleOwned,
					},
					status: types.StackStatusReviewInProgress,
					HTTP2:  true,
				},
				{
					Name:            "managed-stack",
					DNSName:         "example.com",
					TargetGroupARNs: []string{"tg-arn"},
					CertificateARNs: map[string]time.Time{},
					tags: map[string]string{
						kubernetesCreatorTag:                DefaultControllerID,
						clusterIDTagPrefix + "test-cluster": resourceLifecycleOwned,
					},
					status: types.StackStatusRollbackComplete,
					HTTP2:  true,
				},
			},
			wantErr: false,
		},
		{
			"failed-paging",
			fake.CFOutputs{
				DescribeStacks: fake.R(&cloudformation.DescribeStacksOutput{}, nil),
			},
			[]*Stack{},
			true,
		},
		{
			"failed-describe-page",
			fake.CFOutputs{
				DescribeStacks: fake.R(nil, fake.ErrDummy),
			},
			nil,
			true,
		},
	} {
		t.Run(ti.name, func(t *testing.T) {
			c := &fake.CFClient{Outputs: ti.given}
			got, err := findManagedStacks(context.Background(), c, "test-cluster", DefaultControllerID)
			if err != nil {
				if !ti.wantErr {
					t.Error("unexpected error", err)
				}
			} else {
				if !reflect.DeepEqual(ti.want, got) {
					t.Errorf("unexpected result. wanted %+v, got %+v", ti.want, got)
				}
			}
		})
	}
}

func TestGetStack(t *testing.T) {
	for _, ti := range []struct {
		name    string
		given   fake.CFOutputs
		want    *Stack
		wantErr bool
	}{
		{
			name: "successful-call",
			given: fake.CFOutputs{
				DescribeStacks: fake.R(&cloudformation.DescribeStacksOutput{
					Stacks: []types.Stack{
						{
							StackName:   aws.String("managed-stack"),
							StackStatus: types.StackStatusCreateComplete,
							Tags: []types.Tag{
								cfTag(kubernetesCreatorTag, DefaultControllerID),
								cfTag(clusterIDTagPrefix+"test-cluster", resourceLifecycleOwned),
								cfTag(certificateARNTagPrefix+"cert-arn", time.Time{}.Format(time.RFC3339)),
							},
							Outputs: []types.Output{
								{OutputKey: aws.String(outputLoadBalancerDNSName), OutputValue: aws.String("example.com")},
								{OutputKey: aws.String(outputTargetGroupARN), OutputValue: aws.String("tg-arn")},
							},
						},
					},
				}, nil),
			},
			want: &Stack{
				Name:    "managed-stack",
				DNSName: "example.com",
				CertificateARNs: map[string]time.Time{
					"cert-arn": {},
				},
				TargetGroupARNs: []string{"tg-arn"},
				tags: map[string]string{
					kubernetesCreatorTag:                 DefaultControllerID,
					clusterIDTagPrefix + "test-cluster":  resourceLifecycleOwned,
					certificateARNTagPrefix + "cert-arn": time.Time{}.Format(time.RFC3339),
				},
				status: types.StackStatusCreateComplete,
				HTTP2:  true,
			},
			wantErr: false,
		},
		{
			name: "successful-call-http-arn",
			given: fake.CFOutputs{
				DescribeStacks: fake.R(&cloudformation.DescribeStacksOutput{
					Stacks: []types.Stack{
						{
							StackName:   aws.String("managed-stack"),
							StackStatus: types.StackStatusCreateComplete,
							Tags: []types.Tag{
								cfTag(kubernetesCreatorTag, DefaultControllerID),
								cfTag(clusterIDTagPrefix+"test-cluster", resourceLifecycleOwned),
								cfTag(certificateARNTagPrefix+"cert-arn", time.Time{}.Format(time.RFC3339)),
							},
							Outputs: []types.Output{
								{OutputKey: aws.String(outputLoadBalancerDNSName), OutputValue: aws.String("example.com")},
								{OutputKey: aws.String(outputTargetGroupARN), OutputValue: aws.String("tg-arn")},
								{OutputKey: aws.String(outputHTTPTargetGroupARN), OutputValue: aws.String("tg-http-arn")},
							},
						},
					},
				}, nil),
			},
			want: &Stack{
				Name:    "managed-stack",
				DNSName: "example.com",
				CertificateARNs: map[string]time.Time{
					"cert-arn": {},
				},
				TargetGroupARNs: []string{"tg-arn", "tg-http-arn"},
				tags: map[string]string{
					kubernetesCreatorTag:                 DefaultControllerID,
					clusterIDTagPrefix + "test-cluster":  resourceLifecycleOwned,
					certificateARNTagPrefix + "cert-arn": time.Time{}.Format(time.RFC3339),
				},
				status: types.StackStatusCreateComplete,
				HTTP2:  true,
			},
			wantErr: false,
		},
		{
			name: "no-ready-stacks",
			given: fake.CFOutputs{
				DescribeStacks: fake.R(&cloudformation.DescribeStacksOutput{
					Stacks: []types.Stack{},
				}, nil),
			},
			want:    nil,
			wantErr: true,
		},
		{
			"failed-paging",
			fake.CFOutputs{
				DescribeStacks: fake.R(&cloudformation.DescribeStacksOutput{}, nil),
			},
			nil,
			true,
		},
		{
			"failed-describe-page",
			fake.CFOutputs{
				DescribeStacks: fake.R(nil, fake.ErrDummy),
			},
			nil,
			true,
		},
	} {
		t.Run(ti.name, func(t *testing.T) {
			c := &fake.CFClient{Outputs: ti.given}
			s, err := getStack(context.Background(), c, "dontcare")
			if err != nil {
				if !ti.wantErr {
					t.Error("unexpected error", err)
				}
			} else {
				if !reflect.DeepEqual(ti.want, s) {
					t.Errorf("unexpected result. wanted %+v, got %+v", ti.want, s)
				}
			}
		})
	}
}

func TestGetLoadBalancerStackResource(t *testing.T) {

	for _, ti := range []struct {
		name    string
		given   fake.CFOutputs
		want    *types.StackResourceSummary
		wantErr bool
	}{
		{
			"managed load balancer resource found",
			fake.CFOutputs{
				ListStackResources: fake.R(&cloudformation.ListStackResourcesOutput{
					StackResourceSummaries: []types.StackResourceSummary{
						{
							LogicalResourceId:    aws.String(loadBalancerResourceLogicalID),
							ResourceType:         aws.String("AWS::ElasticLoadBalancingV2::LoadBalancer"),
							ResourceStatus:       types.ResourceStatusCreateComplete,
							ResourceStatusReason: aws.String(""),
						},
					}}, nil),
			},
			&types.StackResourceSummary{
				LogicalResourceId:    aws.String(loadBalancerResourceLogicalID),
				ResourceType:         aws.String("AWS::ElasticLoadBalancingV2::LoadBalancer"),
				ResourceStatus:       types.ResourceStatusCreateComplete,
				ResourceStatusReason: aws.String(""),
			},
			false,
		},
		{
			"no load balancer resource",
			fake.CFOutputs{
				ListStackResources: fake.R(&cloudformation.ListStackResourcesOutput{
					StackResourceSummaries: []types.StackResourceSummary{
						{
							LogicalResourceId:    aws.String("SomeOtherResourceLogicalID"),
							ResourceType:         aws.String("AWS::Some::OtherResource"),
							ResourceStatus:       types.ResourceStatusCreateComplete,
							ResourceStatusReason: aws.String(""),
						},
					}}, nil),
			},
			nil,
			true,
		},
		{
			"unmanaged load balancer",
			fake.CFOutputs{
				ListStackResources: fake.R(&cloudformation.ListStackResourcesOutput{
					StackResourceSummaries: []types.StackResourceSummary{
						{
							LogicalResourceId:    aws.String("unmanaged-load-balancer"),
							ResourceType:         aws.String("AWS::ElasticLoadBalancingV2::LoadBalancer"),
							ResourceStatus:       types.ResourceStatusCreateComplete,
							ResourceStatusReason: aws.String(""),
						},
					}}, nil),
			},
			nil,
			true,
		},
	} {
		t.Run(ti.name, func(t *testing.T) {
			c := &fake.CFClient{Outputs: ti.given}
			got, err := getLoadBalancerStackResource(context.Background(), c, "dontcare")
			if err != nil {
				if !ti.wantErr {
					t.Error("unexpected error", err)
				}
			} else {
				if !reflect.DeepEqual(ti.want, got) {
					t.Errorf("unexpected result. wanted %+v, got %+v", ti.want, got)
				}
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
			&Stack{CertificateARNs: map[string]time.Time{"test-arn": time.Now().UTC().Add(1 * time.Minute)}},
			false,
		},
		{
			"DeleteInProgressSecond",
			&Stack{CertificateARNs: map[string]time.Time{"test-arn": time.Now().UTC().Add(1 * time.Second)}},
			false,
		},
		{
			"ShouldDelete",
			&Stack{CertificateARNs: map[string]time.Time{"test-arn": time.Now().UTC().Add(-1 * time.Second)}},
			true,
		},
		{
			"ShouldDeleteMinute",
			&Stack{CertificateARNs: map[string]time.Time{"test-arn": time.Now().UTC().Add(-1 * time.Minute)}},
			true,
		},
		{
			"EmptyStack",
			&Stack{},
			true,
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
