package aws

import (
	"reflect"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
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
			"stack extra listeners",
			stackSpec{
				name:             "foo",
				securityGroupID:  "bar",
				vpcID:            "baz",
				loadbalancerType: LoadBalancerTypeNetwork,
				extraListeners:   []ExtraListener{{ListenProtocol: "TCP", TargetPort: 2222, ListenPort: 22, PodLabel: "app=test"}},
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
		{
			"stack extra listeners",
			stackSpec{
				name:             "foo",
				securityGroupID:  "bar",
				vpcID:            "baz",
				loadbalancerType: LoadBalancerTypeNetwork,
				extraListeners:   []ExtraListener{{ListenProtocol: "TCP", TargetPort: 2222, ListenPort: 22, PodLabel: "app=test"}},
			},
			fake.CFOutputs{UpdateStack: fake.R(fake.MockUSOutput("fake-stack-id"), nil)},
			"fake-stack-id",
			false,
		},
	} {
		t.Run(ti.name, func(t *testing.T) {
			c := &fake.CFClient{Outputs: ti.givenOutputs}
			got, err := updateStack(c, &ti.givenSpec)
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

func Test_lbARN(t *testing.T) {
	t.Run("should return the ARN", func(t *testing.T) {
		want := "blah"
		s := stackOutput{outputLoadBalancerARN: want}
		got := s.lbARN()
		if want != got {
			t.Errorf("unexpected result. wanted %+v, got %+v", want, got)
		}
	})
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
			err := deleteStack(c, ti.givenSpec.name)
			haveErr := err != nil
			if haveErr != ti.wantErr {
				t.Errorf("unexpected result from %s. wanted error %v, got err: %+v", ti.msg, ti.wantErr, err)
			}
		})
	}
}

func TestIsComplete(t *testing.T) {
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
		{cloudformation.StackStatusRollbackComplete, true},
		{cloudformation.StackStatusRollbackFailed, false},
		{cloudformation.StackStatusRollbackInProgress, false},
		{cloudformation.StackStatusUpdateCompleteCleanupInProgress, false},
		{cloudformation.StackStatusUpdateRollbackCompleteCleanupInProgress, false},
		{"dummy-status", false},
	} {
		t.Run(ti.given, func(t *testing.T) {
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
		{stack: &Stack{status: cloudformation.StackStatusCreateInProgress}, expectedError: NONE},
		{stack: &Stack{status: cloudformation.StackStatusCreateComplete}, expectedError: NONE},
		{stack: &Stack{status: cloudformation.StackStatusUpdateInProgress}, expectedError: NONE},
		{stack: &Stack{status: cloudformation.StackStatusUpdateComplete}, expectedError: NONE},
		{stack: &Stack{status: cloudformation.StackStatusUpdateCompleteCleanupInProgress}, expectedError: NONE},
		{stack: &Stack{status: cloudformation.StackStatusDeleteInProgress}, expectedError: NONE},
		{stack: &Stack{status: cloudformation.StackStatusDeleteComplete}, expectedError: NONE},
		//
		{stack: &Stack{status: cloudformation.StackStatusUpdateRollbackComplete}, expectedError: "unexpected status UPDATE_ROLLBACK_COMPLETE"},
		{
			stack: &Stack{
				status:       cloudformation.StackStatusUpdateRollbackInProgress,
				statusReason: "Parameter validation failed: parameter value sg-xxx for parameter name LoadBalancerSecurityGroupParameter does not exist",
			},
			expectedError: "unexpected status UPDATE_ROLLBACK_IN_PROGRESS: Parameter validation failed: parameter value sg-xxx for parameter name LoadBalancerSecurityGroupParameter does not exist",
		},
	} {
		testName := "nil stack"
		if ti.stack != nil {
			testName = ti.stack.status
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
		given []*cloudformation.Tag
		want  bool
	}{
		{"managed", []*cloudformation.Tag{
			cfTag(kubernetesCreatorTag, DefaultControllerID),
			cfTag(clusterIDTagPrefix+"test-cluster", resourceLifecycleOwned),
			cfTag("foo", "bar"),
		}, true},
		{"missing-cluster-tag", []*cloudformation.Tag{
			cfTag(kubernetesCreatorTag, DefaultControllerID),
		}, false},
		{"missing-kube-mgmt-tag", []*cloudformation.Tag{
			cfTag(clusterIDTagPrefix+"test-cluster", resourceLifecycleOwned),
		}, false},
		{"missing-all-mgmt-tag", []*cloudformation.Tag{
			cfTag("foo", "bar"),
		}, false},
		{"mismatch-cluster-tag", []*cloudformation.Tag{
			cfTag(kubernetesCreatorTag, DefaultControllerID),
			cfTag(clusterIDTagPrefix+"other-cluster", resourceLifecycleOwned),
			cfTag("foo", "bar"),
		}, false},
		{"mismatch-controller-id", []*cloudformation.Tag{
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

func TestConvertStackParameters(t *testing.T) {
	for _, ti := range []struct {
		name  string
		given []*cloudformation.Parameter
		want  map[string]string
	}{
		{"default", []*cloudformation.Parameter{
			{
				ParameterKey:   aws.String("foo"),
				ParameterValue: aws.String("bar"),
			},
		}, map[string]string{"foo": "bar"}},
		{"empty-input", []*cloudformation.Parameter{}, map[string]string{}},
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
				DescribeStackPages: fake.R(nil, nil),
				DescribeStacks: fake.R(&cloudformation.DescribeStacksOutput{
					Stacks: []*cloudformation.Stack{
						{
							StackName:   aws.String("managed-stack-not-ready"),
							StackStatus: aws.String(cloudformation.StackStatusUpdateInProgress),
							Tags: []*cloudformation.Tag{
								cfTag(kubernetesCreatorTag, DefaultControllerID),
								cfTag(clusterIDTagPrefix+"test-cluster", resourceLifecycleOwned),
								cfTag(certificateARNTagPrefix+"cert-arn", time.Time{}.Format(time.RFC3339)),
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
								cfTag(kubernetesCreatorTag, DefaultControllerID),
								cfTag(clusterIDTagPrefix+"test-cluster", resourceLifecycleOwned),
								cfTag(certificateARNTagPrefix+"cert-arn", time.Time{}.Format(time.RFC3339)),
							},
							Outputs: []*cloudformation.Output{
								{OutputKey: aws.String(outputLoadBalancerDNSName), OutputValue: aws.String("example.com")},
								{OutputKey: aws.String(outputTargetGroupARN), OutputValue: aws.String("tg-arn")},
							},
						},
						{
							StackName:   aws.String("managed-stack-http-arn"),
							StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
							Tags: []*cloudformation.Tag{
								cfTag(kubernetesCreatorTag, DefaultControllerID),
								cfTag(clusterIDTagPrefix+"test-cluster", resourceLifecycleOwned),
								cfTag(certificateARNTagPrefix+"cert-arn", time.Time{}.Format(time.RFC3339)),
							},
							Outputs: []*cloudformation.Output{
								{OutputKey: aws.String(outputLoadBalancerDNSName), OutputValue: aws.String("example.com")},
								{OutputKey: aws.String(outputTargetGroupARN), OutputValue: aws.String("tg-arn")},
								{OutputKey: aws.String(outputHTTPTargetGroupARN), OutputValue: aws.String("http-tg-arn")},
							},
						},
						{
							StackName:   aws.String("managed-stack-not-ready"),
							StackStatus: aws.String(cloudformation.StackStatusUpdateInProgress),
							Tags: []*cloudformation.Tag{
								cfTag(kubernetesCreatorTag, DefaultControllerID),
								cfTag(clusterIDTagPrefix+"test-cluster", resourceLifecycleOwned),
							},
						},
						{
							StackName: aws.String("unmanaged-stack"),
							Tags: []*cloudformation.Tag{
								cfTag(clusterIDTagPrefix+"test-cluster", resourceLifecycleOwned),
							},
						},
						{
							StackName: aws.String("another-unmanaged-stack"),
							Tags: []*cloudformation.Tag{
								cfTag(kubernetesCreatorTag, DefaultControllerID),
							},
						},
						{
							StackName: aws.String("belongs-to-other-cluster"),
							Tags: []*cloudformation.Tag{
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
					status: cloudformation.StackStatusUpdateInProgress,
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
					status: cloudformation.StackStatusCreateComplete,
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
					status: cloudformation.StackStatusCreateComplete,
					HTTP2:  true,
				},
				{
					Name:            "managed-stack-not-ready",
					CertificateARNs: map[string]time.Time{},
					tags: map[string]string{
						kubernetesCreatorTag:                DefaultControllerID,
						clusterIDTagPrefix + "test-cluster": resourceLifecycleOwned,
					},
					status: cloudformation.StackStatusUpdateInProgress,
					HTTP2:  true,
				},
			},
			wantErr: false,
		},
		{
			name: "no-ready-stacks",
			given: fake.CFOutputs{
				DescribeStackPages: fake.R(nil, nil),
				DescribeStacks: fake.R(&cloudformation.DescribeStacksOutput{
					Stacks: []*cloudformation.Stack{
						{
							StackName:   aws.String("managed-stack-not-ready"),
							StackStatus: aws.String(cloudformation.StackStatusReviewInProgress),
							Tags: []*cloudformation.Tag{
								cfTag(kubernetesCreatorTag, DefaultControllerID),
								cfTag(clusterIDTagPrefix+"test-cluster", resourceLifecycleOwned),
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
								cfTag(kubernetesCreatorTag, DefaultControllerID),
								cfTag(clusterIDTagPrefix+"test-cluster", resourceLifecycleOwned),
							},
							Outputs: []*cloudformation.Output{
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
					status: cloudformation.StackStatusReviewInProgress,
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
					status: cloudformation.StackStatusRollbackComplete,
					HTTP2:  true,
				},
			},
			wantErr: false,
		},
		{
			"failed-paging",
			fake.CFOutputs{
				DescribeStackPages: fake.R(nil, fake.ErrDummy),
				DescribeStacks:     fake.R(&cloudformation.DescribeStacksOutput{}, nil),
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
			got, err := findManagedStacks(c, "test-cluster", DefaultControllerID)
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
				DescribeStackPages: fake.R(nil, nil),
				DescribeStacks: fake.R(&cloudformation.DescribeStacksOutput{
					Stacks: []*cloudformation.Stack{
						{
							StackName:   aws.String("managed-stack"),
							StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
							Tags: []*cloudformation.Tag{
								cfTag(kubernetesCreatorTag, DefaultControllerID),
								cfTag(clusterIDTagPrefix+"test-cluster", resourceLifecycleOwned),
								cfTag(certificateARNTagPrefix+"cert-arn", time.Time{}.Format(time.RFC3339)),
							},
							Outputs: []*cloudformation.Output{
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
				status: cloudformation.StackStatusCreateComplete,
				HTTP2:  true,
			},
			wantErr: false,
		},
		{
			name: "successful-call-http-arn",
			given: fake.CFOutputs{
				DescribeStackPages: fake.R(nil, nil),
				DescribeStacks: fake.R(&cloudformation.DescribeStacksOutput{
					Stacks: []*cloudformation.Stack{
						{
							StackName:   aws.String("managed-stack"),
							StackStatus: aws.String(cloudformation.StackStatusCreateComplete),
							Tags: []*cloudformation.Tag{
								cfTag(kubernetesCreatorTag, DefaultControllerID),
								cfTag(clusterIDTagPrefix+"test-cluster", resourceLifecycleOwned),
								cfTag(certificateARNTagPrefix+"cert-arn", time.Time{}.Format(time.RFC3339)),
							},
							Outputs: []*cloudformation.Output{
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
				status: cloudformation.StackStatusCreateComplete,
				HTTP2:  true,
			},
			wantErr: false,
		},
		{
			name: "no-ready-stacks",
			given: fake.CFOutputs{
				DescribeStackPages: fake.R(nil, nil),
				DescribeStacks: fake.R(&cloudformation.DescribeStacksOutput{
					Stacks: []*cloudformation.Stack{},
				}, nil),
			},
			want:    nil,
			wantErr: true,
		},
		{
			"failed-paging",
			fake.CFOutputs{
				DescribeStackPages: fake.R(nil, fake.ErrDummy),
				DescribeStacks:     fake.R(&cloudformation.DescribeStacksOutput{}, nil),
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
			s, err := getStack(c, "dontcare")
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
