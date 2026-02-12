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
		name                      string
		annotations               map[string]string
		expectedSSLPolicy         string
		expectedHasAnnotation     bool
	}{
		{
			name:                  "no SSL policy annotation - uses default",
			annotations:           map[string]string{},
			expectedSSLPolicy:     defaultSSLPolicy,
			expectedHasAnnotation: false,
		},
		{
			name: "explicit SSL policy annotation",
			annotations: map[string]string{
				ingressSSLPolicyAnnotation: customSSLPolicy,
			},
			expectedSSLPolicy:     customSSLPolicy,
			expectedHasAnnotation: true,
		},
		{
			name: "invalid SSL policy annotation - falls back to default",
			annotations: map[string]string{
				ingressSSLPolicyAnnotation: "InvalidPolicy",
			},
			expectedSSLPolicy:     defaultSSLPolicy,
			expectedHasAnnotation: false,
		},
		{
			name: "valid SSL policy same as default - still marked as annotation",
			annotations: map[string]string{
				ingressSSLPolicyAnnotation: defaultSSLPolicy,
			},
			expectedSSLPolicy:     defaultSSLPolicy,
			expectedHasAnnotation: true,
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
			assert.Equal(t, tt.expectedHasAnnotation, result.HasSSLPolicyAnnotation, "HasSSLPolicyAnnotation mismatch")
		})
	}
}
