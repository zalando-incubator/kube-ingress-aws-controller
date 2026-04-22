package kubernetes

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zalando-incubator/kube-ingress-aws-controller/aws"
)

func TestHasSSLPolicyAnnotation(t *testing.T) {
	defaultSSLPolicy := "ELBSecurityPolicy-2016-08"
	customSSLPolicy := "ELBSecurityPolicy-TLS-1-2-2017-01"

	tests := []struct {
		name                string
		annotations         map[string]string
		expectedSSLPolicy   string
		expectedSharedState bool
	}{
		{
			name:                "no SSL policy annotation - uses default and marked as shared",
			annotations:         map[string]string{},
			expectedSSLPolicy:   defaultSSLPolicy,
			expectedSharedState: true,
		},
		{
			name: "explicit non-defaultSSL policy annotation",
			annotations: map[string]string{
				ingressSSLPolicyAnnotation: customSSLPolicy,
			},
			expectedSSLPolicy:   customSSLPolicy,
			expectedSharedState: false,
		},
		{
			name: "invalid SSL policy annotation - falls back to default and marked as shared",
			annotations: map[string]string{
				ingressSSLPolicyAnnotation: "InvalidPolicy",
			},
			expectedSSLPolicy:   defaultSSLPolicy,
			expectedSharedState: true,
		},
		{
			name: "valid SSL policy same as default - marked as shared",
			annotations: map[string]string{
				ingressSSLPolicyAnnotation: defaultSSLPolicy,
			},
			expectedSSLPolicy:   defaultSSLPolicy,
			expectedSharedState: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter, err := NewAdapter(
				testConfig,
				IngressAPIVersionNetworking,
				[]string{},
				"sg-123",
				defaultSSLPolicy,
				aws.LoadBalancerTypeApplication,
				"",
				aws.IPAddressTypeIPV4,
				false,
			)
			assert.NoError(t, err)

			kubeIngress := &ingress{
				Metadata: kubeItemMetadata{
					Namespace:   "default",
					Name:        "test-ingress",
					Annotations: tt.annotations,
				},
				Spec: ingressSpec{
					Rules: []ingressItemRule{
						{Host: "example.com"},
					},
				},
			}

			result, err := adapter.newIngressFromKube(kubeIngress)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedSSLPolicy, result.SSLPolicy, "SSL policy mismatch")
			assert.Equal(t, tt.expectedSharedState, result.Shared, "Shared state mismatch")
		})
	}
}
