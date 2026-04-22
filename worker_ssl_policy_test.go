package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/zalando-incubator/kube-ingress-aws-controller/aws"
	"github.com/zalando-incubator/kube-ingress-aws-controller/kubernetes"
)

func TestSSLPolicyInSync(t *testing.T) {
	tests := []struct {
		name           string
		loadBalancer   *loadBalancer
		expectedInSync bool
		description    string
	}{
		{
			name: "in sync when SSL policies match",
			loadBalancer: &loadBalancer{
				sslPolicy: "ELBSecurityPolicy-TLS-1-2-2017-01",
				ingresses: map[string][]*kubernetes.Ingress{
					"cert-1": {
						&kubernetes.Ingress{
							Namespace: "default",
							Name:      "test-ingress",
						},
					},
				},
				stack: &aws.Stack{
					SSLPolicy: "ELBSecurityPolicy-TLS-1-2-2017-01",
					CertificateARNs: map[string]time.Time{
						"cert-1": {},
					},
				},
				cwAlarms: aws.CloudWatchAlarmList{},
			},
			expectedInSync: true,
			description:    "Load balancer should be in sync when SSL policies match",
		},
		{
			name: "out of sync when SSL policies differ",
			loadBalancer: &loadBalancer{
				sslPolicy: "ELBSecurityPolicy-TLS-1-2-2017-01",
				ingresses: map[string][]*kubernetes.Ingress{
					"cert-1": {
						&kubernetes.Ingress{
							Namespace: "default",
							Name:      "test-ingress",
						},
					},
				},
				stack: &aws.Stack{
					SSLPolicy: "ELBSecurityPolicy-2016-08",
					CertificateARNs: map[string]time.Time{
						"cert-1": {},
					},
				},
				cwAlarms: aws.CloudWatchAlarmList{},
			},
			expectedInSync: false,
			description:    "Load balancer should be out of sync when SSL policies differ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.loadBalancer.inSync()
			assert.Equal(t, tt.expectedInSync, result, tt.description)
		})
	}
}
