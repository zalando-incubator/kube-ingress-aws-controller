package main

import (
	"crypto/x509"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
	} {
		tt.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.loadBalancer.AddIngress(test.certificateARNs, test.ingress, test.maxCerts), test.added)
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
