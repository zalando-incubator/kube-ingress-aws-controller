package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/zalando-incubator/kube-ingress-aws-controller/aws"
	"github.com/zalando-incubator/kube-ingress-aws-controller/kubernetes"
)

func TestSSLPolicyAnnotationLoadBalancerMatching(t *testing.T) {
	tests := []struct {
		name                      string
		existingLB                *loadBalancer
		incomingIngress           *kubernetes.Ingress
		maxCerts                  int
		shouldAdd                 bool
		description               string
	}{
		{
			name: "shared LB - no annotation - should share despite different policy",
			existingLB: &loadBalancer{
				shared:    true,
				sslPolicy: "ELBSecurityPolicy-2016-08",
				ingresses: make(map[string][]*kubernetes.Ingress),
				scheme:    "internet-facing",
				loadBalancerType: aws.LoadBalancerTypeApplication,
			},
			incomingIngress: &kubernetes.Ingress{
				SSLPolicy:              "ELBSecurityPolicy-TLS-1-2-2017-01",
				HasSSLPolicyAnnotation: false, // Using default from command line
				Shared:                 true,
				Scheme:                 "internet-facing",
				LoadBalancerType:       aws.LoadBalancerTypeApplication,
			},
			maxCerts:    10,
			shouldAdd:   true,
			description: "Ingress without annotation should share LB even with different SSL policy",
		},
		{
			name: "shared LB - with annotation - matching policy - should share",
			existingLB: &loadBalancer{
				shared:    true,
				sslPolicy: "ELBSecurityPolicy-TLS-1-2-2017-01",
				ingresses: make(map[string][]*kubernetes.Ingress),
				scheme:    "internet-facing",
				loadBalancerType: aws.LoadBalancerTypeApplication,
			},
			incomingIngress: &kubernetes.Ingress{
				SSLPolicy:              "ELBSecurityPolicy-TLS-1-2-2017-01",
				HasSSLPolicyAnnotation: true, // Explicit annotation
				Shared:                 true,
				Scheme:                 "internet-facing",
				LoadBalancerType:       aws.LoadBalancerTypeApplication,
			},
			maxCerts:    10,
			shouldAdd:   true,
			description: "Ingress with matching SSL policy annotation should share LB",
		},
		{
			name: "shared LB - with annotation - different policy - should NOT share",
			existingLB: &loadBalancer{
				shared:    true,
				sslPolicy: "ELBSecurityPolicy-2016-08",
				ingresses: make(map[string][]*kubernetes.Ingress),
				scheme:    "internet-facing",
				loadBalancerType: aws.LoadBalancerTypeApplication,
			},
			incomingIngress: &kubernetes.Ingress{
				SSLPolicy:              "ELBSecurityPolicy-TLS-1-2-2017-01",
				HasSSLPolicyAnnotation: true, // Explicit annotation
				Shared:                 true,
				Scheme:                 "internet-facing",
				LoadBalancerType:       aws.LoadBalancerTypeApplication,
			},
			maxCerts:    10,
			shouldAdd:   false,
			description: "Ingress with different SSL policy annotation should NOT share LB",
		},
		{
			name: "non-shared LB - always allows different SSL policies",
			existingLB: &loadBalancer{
				shared:    false,
				sslPolicy: "ELBSecurityPolicy-2016-08",
				ingresses: make(map[string][]*kubernetes.Ingress),
				scheme:    "internet-facing",
				loadBalancerType: aws.LoadBalancerTypeApplication,
				stack: &aws.Stack{
					OwnerIngress: "default/test",
				},
			},
			incomingIngress: &kubernetes.Ingress{
				Namespace:              "default",
				Name:                   "test",
				SSLPolicy:              "ELBSecurityPolicy-TLS-1-2-2017-01",
				HasSSLPolicyAnnotation: true,
				Shared:                 false,
				Scheme:                 "internet-facing",
				LoadBalancerType:       aws.LoadBalancerTypeApplication,
			},
			maxCerts:    10,
			shouldAdd:   true,
			description: "Non-shared LB can update SSL policy regardless of annotation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.existingLB.addIngress([]string{"cert-arn-1"}, tt.incomingIngress, tt.maxCerts)
			assert.Equal(t, tt.shouldAdd, result, tt.description)
		})
	}
}

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
