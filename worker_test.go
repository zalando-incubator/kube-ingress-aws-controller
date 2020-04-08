package main

import (
	"crypto/x509"
	"testing"
	"time"

	cloudformation "github.com/mweagle/go-cloudformation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zalando-incubator/kube-ingress-aws-controller/aws"
	"github.com/zalando-incubator/kube-ingress-aws-controller/certs"
	"github.com/zalando-incubator/kube-ingress-aws-controller/kubernetes"
)

func TestAddIngress(tt *testing.T) {
	for _, test := range []struct {
		name            string
		loadBalancer    *loadBalancer
		certificateARNs []string
		ingress         *kubernetes.Ingress
		maxCerts        int
		added           bool
	}{
		{
			name: "scheme not matching",
			loadBalancer: &loadBalancer{
				scheme: "foo",
			},
			ingress: &kubernetes.Ingress{
				Scheme: "bar",
			},
			added: false,
		},
		{
			name: "security group not matching",
			loadBalancer: &loadBalancer{
				securityGroup: "foo",
			},
			ingress: &kubernetes.Ingress{
				SecurityGroup: "bar",
			},
			added: false,
		},
		{
			name: "ip address type not matching",
			loadBalancer: &loadBalancer{
				ipAddressType: aws.IPAddressTypeIPV4,
			},
			ingress: &kubernetes.Ingress{
				IPAddressType: aws.IPAddressTypeDualstack,
			},
			added: false,
		},
		{
			name: "don't add ingresses non-shared, non-owned load balancer",
			loadBalancer: &loadBalancer{
				stack: &aws.Stack{
					OwnerIngress: "foo/bar",
				},
			},
			ingress: &kubernetes.Ingress{
				Shared:    false,
				Namespace: "foo",
				Name:      "baz",
			},
			added: false,
		},
		{
			name: "don't add ingresses shared, to an owned load balancer",
			loadBalancer: &loadBalancer{
				stack: &aws.Stack{
					OwnerIngress: "foo/bar",
				},
			},
			ingress: &kubernetes.Ingress{
				Shared:    true,
				Namespace: "foo",
				Name:      "baz",
			},
			added: false,
		},
		{
			name: "add ingress to empty load balancer",
			loadBalancer: &loadBalancer{
				stack: &aws.Stack{},
				ingresses: map[string][]*kubernetes.Ingress{
					"foo": []*kubernetes.Ingress{
						{
							Shared: true,
						},
					},
				},
			},
			certificateARNs: []string{"foo", "bar", "baz"},
			ingress: &kubernetes.Ingress{
				Shared: true,
			},
			maxCerts: 5,
			added:    true,
		},
		{
			name: "fail adding when too many certs",
			loadBalancer: &loadBalancer{
				stack: &aws.Stack{},
				ingresses: map[string][]*kubernetes.Ingress{
					"foo": []*kubernetes.Ingress{
						{
							Shared: true,
						},
					},
				},
			},
			certificateARNs: []string{"foo", "bar"},
			ingress: &kubernetes.Ingress{
				Shared: true,
			},
			maxCerts: 1,
			added:    false,
		},
		{
			name: "wafacl id not matching",
			loadBalancer: &loadBalancer{
				wafWebACLId: "WAFZXX",
			},
			ingress: &kubernetes.Ingress{
				WAFWebACLId: "WAFZXC",
			},
			added: false,
		},
		{
			name: "wafacl id not matching",
			loadBalancer: &loadBalancer{
				wafWebACLId: aws.DefaultWAFWebAclId,
			},
			ingress: &kubernetes.Ingress{
				WAFWebACLId: "WAFZXC",
			},
			added: false,
		},
	} {
		tt.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.loadBalancer.addIngress(test.certificateARNs, test.ingress, test.maxCerts), test.added)
		})
	}
}

func TestSortStacks(tt *testing.T) {
	testTime := time.Now()

	for _, test := range []struct {
		name           string
		stacks         []*aws.Stack
		expectedStacks []*aws.Stack
	}{
		{
			name:           "no stacks",
			stacks:         []*aws.Stack{},
			expectedStacks: []*aws.Stack{},
		},
		{
			name: "two unsorted stacks",
			stacks: []*aws.Stack{
				&aws.Stack{
					Name:            "foo",
					CertificateARNs: map[string]time.Time{},
				},
				&aws.Stack{
					Name: "bar",
					CertificateARNs: map[string]time.Time{
						"cert-arn": testTime,
					},
				},
			},
			expectedStacks: []*aws.Stack{
				&aws.Stack{
					Name: "bar",
					CertificateARNs: map[string]time.Time{
						"cert-arn": testTime,
					},
				},
				&aws.Stack{
					Name:            "foo",
					CertificateARNs: map[string]time.Time{},
				},
			},
		},
		{
			name: "two unsorted stacks with the same amount of certificates",
			stacks: []*aws.Stack{
				&aws.Stack{
					Name: "foo",
					CertificateARNs: map[string]time.Time{
						"different-cert-arn": testTime,
					},
				},
				&aws.Stack{
					Name: "bar",
					CertificateARNs: map[string]time.Time{
						"cert-arn": testTime,
					},
				},
			},
			expectedStacks: []*aws.Stack{
				&aws.Stack{
					Name: "bar",
					CertificateARNs: map[string]time.Time{
						"cert-arn": testTime,
					},
				},
				&aws.Stack{
					Name: "foo",
					CertificateARNs: map[string]time.Time{
						"different-cert-arn": testTime,
					},
				},
			},
		},
	} {
		tt.Run(test.name, func(t *testing.T) {
			sortStacks(test.stacks)

			assert.Equal(t, test.expectedStacks, test.stacks)
		})
	}
}

func TestCertificateSummaries(t *testing.T) {
	certificateSummaries := []*certs.CertificateSummary{&certs.CertificateSummary{}}

	certs := &Certificates{certificateSummaries: certificateSummaries}

	assert.Equal(t, certificateSummaries, certs.CertificateSummaries())
}

func TestCertificateExists(tt *testing.T) {
	existingCertificateARN := "existing-arn"
	nonExistingCertificateARN := "non-existing-arn"

	for _, test := range []struct {
		name                 string
		certificateSummaries []*certs.CertificateSummary
		exists               bool
	}{
		{
			name: "certificate is present",
			certificateSummaries: []*certs.CertificateSummary{
				certs.NewCertificate(
					existingCertificateARN,
					&x509.Certificate{},
					[]*x509.Certificate{&x509.Certificate{}},
				),
			},
			exists: true,
		},
		{
			name: "certificate is not present",
			certificateSummaries: []*certs.CertificateSummary{
				certs.NewCertificate(
					nonExistingCertificateARN,
					&x509.Certificate{},
					[]*x509.Certificate{&x509.Certificate{}},
				),
			},
			exists: false,
		},
	} {
		tt.Run(test.name, func(t *testing.T) {
			certs := &Certificates{certificateSummaries: test.certificateSummaries}

			assert.Equal(t, test.exists, certs.CertificateExists(existingCertificateARN))
		})
	}
}

func TestGetAllLoadBalancers(tt *testing.T) {
	certTTL, _ := time.ParseDuration("90d")

	for _, test := range []struct {
		name          string
		stacks        []*aws.Stack
		loadBalancers []*loadBalancer
	}{
		{
			name: "one stack",
			stacks: []*aws.Stack{
				&aws.Stack{
					Scheme:        "foo",
					SecurityGroup: "sg-123456",
				},
			},
			loadBalancers: []*loadBalancer{
				&loadBalancer{
					securityGroup: "sg-123456",
					scheme:        "foo",
					shared:        true,
					ingresses:     map[string][]*kubernetes.Ingress{},
					certTTL:       certTTL,
				},
			},
		},
	} {
		tt.Run(test.name, func(t *testing.T) {
			for i, loadBalancer := range test.loadBalancers {
				loadBalancer.stack = test.stacks[i]
			}

			assert.Equal(t, test.loadBalancers, getAllLoadBalancers(certTTL, test.stacks))
		})
	}
}

func TestGetCloudWatchAlarmConfigFromConfigMap(t *testing.T) {
	for _, test := range []struct {
		name     string
		cm       *kubernetes.ConfigMap
		expected aws.CloudWatchAlarmList
	}{
		{
			name:     "empty config map",
			cm:       &kubernetes.ConfigMap{},
			expected: aws.CloudWatchAlarmList{},
		},
		{
			name: "config map with one data key",
			cm: &kubernetes.ConfigMap{
				Data: map[string]string{
					"some-key": "- AlarmName: foo\n- AlarmName: bar\n",
				},
			},
			expected: aws.CloudWatchAlarmList{
				{AlarmName: cloudformation.String("foo")},
				{AlarmName: cloudformation.String("bar")},
			},
		},
		{
			name: "config map with multiple data keys",
			cm: &kubernetes.ConfigMap{
				Data: map[string]string{
					"some-other-key": "- AlarmName: baz\n",
					"some-key":       "- AlarmName: foo\n- AlarmName: bar\n",
				},
			},
			expected: aws.CloudWatchAlarmList{
				{AlarmName: cloudformation.String("foo")},
				{AlarmName: cloudformation.String("bar")},
				{AlarmName: cloudformation.String("baz")},
			},
		},
		{
			name: "config map with invalid yaml data",
			cm: &kubernetes.ConfigMap{
				Data: map[string]string{
					"some-key": "{",
				},
			},
			expected: aws.CloudWatchAlarmList{},
		},
		{
			name: "config map with partially invalid yaml data",
			cm: &kubernetes.ConfigMap{
				Data: map[string]string{
					"some-key":       "{",
					"some-other-key": "- AlarmName: baz\n",
				},
			},
			expected: aws.CloudWatchAlarmList{
				{AlarmName: cloudformation.String("baz")},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := getCloudWatchAlarmsFromConfigMap(test.cm)

			assert.Equal(t, test.expected, result)
		})
	}
}

func TestAttachCloudWatchAlarmsCopy(t *testing.T) {
	lbOne := &loadBalancer{scheme: "foo"}
	lbTwo := &loadBalancer{scheme: "bar"}

	lbs := []*loadBalancer{
		lbOne,
		lbTwo,
	}

	alarms := aws.CloudWatchAlarmList{
		{AlarmName: cloudformation.String("baz")},
	}

	attachCloudWatchAlarms(lbs, alarms)

	expected := []*loadBalancer{
		{scheme: "foo", cwAlarms: alarms},
		{scheme: "bar", cwAlarms: alarms},
	}

	require.Equal(t, expected, lbs)

	// This should not modify the alarms of lbTwo.
	lbOne.cwAlarms[0].AlarmName = cloudformation.String("qux")

	assert.Equal(t, cloudformation.String("baz"), lbTwo.cwAlarms[0].AlarmName)
}
