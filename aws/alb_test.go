package aws

import (
	"fmt"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/aws/aws-sdk-go/service/elbv2/elbv2iface"
	"github.com/jtblin/aws-mock-metadata/Godeps/_workspace/src/github.com/aws/aws-sdk-go/aws"
	"reflect"
	"testing"
)

func mockSpec(name, certARN, clusterID string) createLoadBalancerSpec {
	return createLoadBalancerSpec{
		name:           name,
		certificateARN: certARN,
		clusterID:      clusterID,
		healthCheck: healthCheck{
			path: "/healthz",
			port: 8080,
		},
		vpcID: "dummy",
	}
}

func TestNormalization(t *testing.T) {
	for _, test := range []struct {
		clusterID string
		arn       string
		want      string
	}{
		{
			"playground",
			"arn:aws:acm:eu-central-1:123456789012:certificate/f4bd7ed6-bf23-11e6-8db1-ef7ba1500c61",
			"playground-7cf27d0",
		},
		{"foo", "", "foo-fd80709"},
		{"foo", "bar", "foo-1f01f4d"},
		{"foo", "fake-cert-arn", "foo-f398861"},
		{"test/with-invalid+chars", "bar", "test-with-invalid-chars-1f01f4d"},
		{"-apparently-valid-name", "bar", "apparently-valid-name-1f01f4d"},
		{"apparently-valid-name-", "bar", "apparently-valid-name-1f01f4d"},
		{"valid--name", "bar", "valid-name-1f01f4d"},
		{"valid-----name", "bar", "valid-name-1f01f4d"},
		{"foo bar baz zbr", "bar", "foo-bar-baz-zbr-1f01f4d"},
		{"very long cluster id needs to be truncated", "bar", "id-needs-to-be-truncated-1f01f4d"},
		{"-no---need---to---truncate-", "", "no-need-to-truncate-fd80709"},
	} {
		t.Run(fmt.Sprintf("%s", test.want), func(t *testing.T) {
			got := normalizeLoadBalancerName(test.clusterID, test.arn)
			if test.want != got {
				t.Errorf("unexpected normalized load balancer name. wanted %q, got %q", test.want, got)
			}
		})
	}

}

func TestCreateLoadBalancer(t *testing.T) {
	dummySpec := mockSpec("foo", "bar", "baz")
	for _, test := range []struct {
		name      string
		spec      createLoadBalancerSpec
		responses elbv2APIOutputs
		want      *LoadBalancer
		wantError bool
	}{
		{
			"success-call", mockSpec("foo", "fake-cert-arn", "bar"),
			elbv2APIOutputs{
				createLoadBalancer: R(mockCLBOutput("domain.com", "fake-lb-arn"), nil),
				createTargetGroup:  R(mockCTGOutput("fake-tg-arn"), nil),
				createListener:     R(mockCLOutput("foo"), nil),
			},
			mockLoadBalancer("bar-f398861", "fake-lb-arn", "domain.com",
				mockListener(443, "crap", "fake-cert-arn", "fake-tg-arn")),
			false,
		},
		{
			"no-listener-result", mockSpec("foo", "", "baz"),
			elbv2APIOutputs{
				createLoadBalancer: R(mockCLBOutput("domain.com", "fake-lb-arn"), nil),
				createTargetGroup:  R(mockCTGOutput("fake-tg-arn"), nil),
				createListener:     R(mockCLOutput("foo"), nil),
			},
			mockLoadBalancer("baz-fd80709", "fake-lb-arn", "domain.com", nil),
			false,
		},
		{
			"fail-create-loadbalancer-aws", dummySpec,
			elbv2APIOutputs{
				createLoadBalancer: R(nil, dummyErr),
			},
			nil, true,
		},
		{
			"success-call-without-lbs", dummySpec,
			elbv2APIOutputs{
				createLoadBalancer: R(&elbv2.CreateLoadBalancerOutput{}, nil),
			},
			nil, true,
		},
		{
			"fail-to-create-targetgroup", dummySpec,
			elbv2APIOutputs{
				createLoadBalancer: R(mockCLBOutput("domain.com", "fake-lb-arn"), nil),
				createTargetGroup:  R(nil, dummyErr),
			},
			nil, true,
		},
		{
			"success-call-without-targetgroups", dummySpec,
			elbv2APIOutputs{
				createLoadBalancer: R(mockCLBOutput("domain.com", "fake-lb-arn"), nil),
				createTargetGroup:  R(&elbv2.CreateTargetGroupOutput{}, nil),
			},
			nil, true,
		},
		{
			"fail-to-create-listener", dummySpec,
			elbv2APIOutputs{
				createLoadBalancer: R(mockCLBOutput("domain.com", "fake-lb-arn"), nil),
				createTargetGroup:  R(mockCTGOutput("fake-tg-arn"), nil),
				createListener:     R(nil, dummyErr),
			},
			nil, true,
		},
		{
			"success-call-without-listeners", dummySpec,
			elbv2APIOutputs{
				createLoadBalancer: R(mockCLBOutput("domain.com", "fake-lb-arn"), nil),
				createTargetGroup:  R(mockCTGOutput("fake-tg-arn"), nil),
				createListener:     R(&elbv2.CreateListenerOutput{}, nil),
			},
			nil, true,
		},
	} {
		t.Run(fmt.Sprintf("%v", test.name), func(t *testing.T) {
			mockSvc := &mockELBV2Client{outputs: test.responses}
			got, err := createLoadBalancer(mockSvc, &test.spec)

			if test.wantError {
				if err == nil {
					t.Error("wanted an error but call seemed to have succeeded")
				}
			} else {
				if err != nil {
					t.Fatal("unexpected error", err)
				}

				if got.Name() != test.want.Name() {
					t.Errorf("unexpected lb name. wanted %q, got %q", test.want.Name(), got.Name())
				}

				if got.DNSName() != test.want.DNSName() {
					t.Errorf("unexpected lb DNS name. wanted %q, got %q", test.want.DNSName(), got.DNSName())
				}

				if got.ARN() != test.want.ARN() {
					t.Errorf("unexpected lb ARN. wanted %q, got %q", test.want.ARN(), got.ARN())
				}

				if got.CertificateARN() != test.want.CertificateARN() {
					t.Errorf("unexpected lb certificate ARN. wanted %q, got %q", test.want.CertificateARN(), got.CertificateARN())
				}
			}
		})
	}
}

func TestFindingManagedLoadBalancers(t *testing.T) {
	for _, test := range []struct {
		name           string
		responses      elbv2APIOutputs
		givenClusterID string
		givenARN       string
		want           *LoadBalancer
		wantError      bool
	}{
		{
			"successful-call",
			elbv2APIOutputs{
				describeLoadBalancers: R(mockDLBOutput(lbMock{"fake-name", "fake-lb-arn", "foo.bar"}), nil),
				describeTags: R(mockDTOutput(awsTags{
					"fake-lb-arn": {
						kubernetesCreatorTag: kubernetesCreatorValue,
						clusterIDTag:         "fake-cluster-id",
					},
				}), nil),
				describeListeners: R(mockDLOutput(443, "fake-arn", "fake-tg-arn", "cert-arn"), nil),
			},
			"fake-cluster-id", "cert-arn",
			mockLoadBalancer("fake-name", "fake-lb-arn", "foo.bar",
				mockListener(443, "fake-arn", "cert-arn", "fake-tg-arn")),
			false,
		},
		{
			"failed-to-describe-load-balancers",
			elbv2APIOutputs{describeLoadBalancers: R(nil, dummyErr)},
			"", "", nil, true,
		},
		{
			"no-load-balancers",
			elbv2APIOutputs{describeLoadBalancers: R(&elbv2.DescribeLoadBalancersOutput{}, nil)},
			"", "", nil, true,
		},
		{
			"failed-to-describe-tags",
			elbv2APIOutputs{
				describeLoadBalancers: R(mockDLBOutput(lbMock{"fake-name", "fake-lb-arn", "foo.bar"}), nil),
				describeTags:          R(nil, dummyErr),
			},
			"", "", nil, true,
		},
		{
			"failed-to-describe-listeners",
			elbv2APIOutputs{
				describeLoadBalancers: R(mockDLBOutput(lbMock{"fake-name", "fake-lb-arn", "foo.bar"}), nil),
				describeTags: R(mockDTOutput(awsTags{
					"fake-lb-arn": {
						kubernetesCreatorTag: kubernetesCreatorValue,
						clusterIDTag:         "fake-cluster-id",
					},
				}), nil),
				describeListeners: R(nil, dummyErr),
			},
			"fake-cluster-id", "",
			nil,
			false,
		},
		{
			"no-secure-listeners",
			elbv2APIOutputs{
				describeLoadBalancers: R(mockDLBOutput(lbMock{"fake-name", "fake-lb-arn", "foo.bar"}), nil),
				describeTags: R(mockDTOutput(awsTags{
					"fake-lb-arn": {
						kubernetesCreatorTag: kubernetesCreatorValue,
						clusterIDTag:         "fake-cluster-id",
					},
				}), nil),
				describeListeners: R(mockDLOutput(80, "fake-arn", "fake-tg-arn", ""), nil),
			},
			"fake-cluster-id", "cert-arn",
			nil,
			false,
		},
		{
			"listener-without-default-action",
			elbv2APIOutputs{
				describeLoadBalancers: R(mockDLBOutput(lbMock{"fake-name", "fake-lb-arn", "foo.bar"}), nil),
				describeTags: R(mockDTOutput(awsTags{
					"fake-lb-arn": {
						kubernetesCreatorTag: kubernetesCreatorValue,
						clusterIDTag:         "fake-cluster-id",
					},
				}), nil),
				describeListeners: R(mockDLOutput(443, "fake-arn", "", "cert-arn"), nil),
			},
			"fake-cluster-id", "cert-arn",
			nil,
			false,
		},
	} {
		t.Run(fmt.Sprintf("%v", test.name), func(t *testing.T) {
			mockSvc := &mockELBV2Client{outputs: test.responses}
			got, err := findManagedLBWithCertificateID(mockSvc, test.givenClusterID, test.givenARN)
			if test.wantError {
				if err == nil {
					t.Error("wanted an error but call seemed to have succeeded")
				}
			} else {
				if err != nil {
					t.Fatal("unexpected error:", err)
				}
				if !reflect.DeepEqual(test.want, got) {
					t.Errorf("unexpected result: %v != %v", *got.listener, *test.want.listener)
				}
			}
		})
	}
}

func TestFindingListeners(t *testing.T) {
	for _, test := range []struct {
		name         string
		given        []*elbv2.Listener
		wantListener *elbv2.Listener
		wantARN      string
	}{
		{"nil-listeners", nil, nil, ""},
		{
			"single-listener",
			mockElbv2Listeners(443, "", "", "foo"),
			mockElbv2Listeners(443, "", "", "foo")[0],
			"foo",
		},
		{
			"multiple-cert-listeners",
			mockElbv2Listeners(443, "", "", "foo", "bar"),
			mockElbv2Listeners(443, "", "", "foo")[0],
			"foo",
		},
		{
			"multiple-mixed-listeners",
			mockElbv2Listeners(443, "", "", "", "bar"),
			mockElbv2Listeners(443, "", "", "bar")[0],
			"bar",
		},
	} {
		t.Run(fmt.Sprintf("%v", test.name), func(t *testing.T) {
			gotListener, gotARN := findFirstListenerWithAnyCertificate(test.given)
			if !reflect.DeepEqual(gotListener, test.wantListener) {
				t.Errorf("unexpected listener. wanted %v, got %v", test.wantListener, gotListener)
			}
			if gotARN != test.wantARN {
				t.Errorf("unexpected ARN. wanted %q, got %q", test.wantARN, gotARN)
			}
		})
	}
}

func TestIdentifyManagedLoadBalancer(t *testing.T) {
	for _, test := range []struct {
		name           string
		givenTags      map[string]string
		givenClusterID string
		want           bool
	}{
		{
			"match",
			map[string]string{
				kubernetesCreatorTag: kubernetesCreatorValue,
				clusterIDTag:         "foo",
			},
			"foo", true,
		},
		{
			"creator-tag-mismatch",
			map[string]string{
				kubernetesCreatorTag: "mismatch",
				clusterIDTag:         "foo",
			},
			"foo", false,
		},
		{
			"cluster-tag-mismatch",
			map[string]string{
				kubernetesCreatorTag: kubernetesCreatorValue,
				clusterIDTag:         "foo",
			},
			"bar", false,
		},
		{
			"all-required-tags-mismatch",
			map[string]string{
				kubernetesCreatorTag: "foo",
				clusterIDTag:         "bar",
			},
			"baz", false,
		},
		{
			"creator-tag-missing",
			map[string]string{clusterIDTag: "foo"},
			"foo", false,
		},
		{
			"cluster-tag-missing",
			map[string]string{kubernetesCreatorTag: kubernetesCreatorValue},
			"foo", false,
		},
		{"all-required-tags-missing", map[string]string{}, "foo", false},
		{"nil-tags", nil, "foo", false},
	} {
		t.Run(fmt.Sprintf("%v", test.name), func(t *testing.T) {
			if got := isManagedLoadBalancer(test.givenTags, test.givenClusterID); got != test.want {
				t.Errorf("failed to identify managed lb for cluster %q with tags %v: result = %v",
					test.givenClusterID, test.givenTags, got)
			}
		})
	}
}

func TestConvertTags(t *testing.T) {
	for _, test := range []struct {
		name  string
		given []*elbv2.Tag
		want  map[string]string
	}{
		{"nil-tags", nil, map[string]string{}},
		{
			"single-tag",
			[]*elbv2.Tag{{Key: aws.String("foo"), Value: aws.String("bar")}},
			map[string]string{"foo": "bar"},
		},
		{
			"multiple-tags",
			[]*elbv2.Tag{
				{Key: aws.String("foo"), Value: aws.String("bar")},
				{Key: aws.String("baz"), Value: aws.String("zbr")},
			},
			map[string]string{"foo": "bar", "baz": "zbr"},
		},
		{
			"repeated-tags",
			[]*elbv2.Tag{
				{Key: aws.String("foo"), Value: aws.String("bar")},
				{Key: aws.String("foo"), Value: aws.String("baz")},
			},
			map[string]string{"foo": "baz"},
		},
	} {
		t.Run(fmt.Sprintf("%v", test.name), func(t *testing.T) {
			got := convertElbv2Tags(test.given)
			if !reflect.DeepEqual(test.want, got) {
				t.Errorf("unexpected result. wanted %v, got %v", test.want, got)
			}
		})
	}
}

func TestDeleteResources(t *testing.T) {
	for _, test := range []struct {
		name      string
		f         func(elbv2iface.ELBV2API, string) error
		responses elbv2APIOutputs
		wantError bool
	}{
		{
			"sucess-delete-loadbalancer", deleteLoadBalancer,
			elbv2APIOutputs{deleteLoadBalancer: R(nil, nil)}, false,
		},
		{
			"fail-delete-loadbalancer", deleteLoadBalancer,
			elbv2APIOutputs{deleteLoadBalancer: R(nil, dummyErr)}, true,
		},
		{
			"sucess-delete-targetgroup", deleteTargetGroup,
			elbv2APIOutputs{deleteTargetGroup: R(nil, nil)}, false,
		},
		{
			"fail-delete-targetgroup", deleteTargetGroup,
			elbv2APIOutputs{deleteTargetGroup: R(nil, dummyErr)}, true,
		},
		{
			"sucess-delete-listener", deleteListener,
			elbv2APIOutputs{deleteListener: R(nil, nil)}, false,
		},
		{
			"fail-delete-listener", deleteListener,
			elbv2APIOutputs{deleteListener: R(nil, dummyErr)}, true,
		},
	} {
		t.Run(fmt.Sprintf("%v", test.name), func(t *testing.T) {
			mockSvc := &mockELBV2Client{outputs: test.responses}
			err := test.f(mockSvc, "dummy")
			if test.wantError {
				if err == nil {
					t.Error("wanted an error but call seemed to have succeeded")
				}
			} else {
				if err != nil {
					t.Error(err)
				}
			}
		})
	}
}
