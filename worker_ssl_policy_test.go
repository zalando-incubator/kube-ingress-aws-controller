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
		name            string
		existingLB      *loadBalancer
		incomingIngress *kubernetes.Ingress
		maxCerts        int
		shouldAdd       bool
		description     string
	}{
		{
			name: "shared LB - explicit policy on LB - no annotation on ingress - different policy - should NOT share",
			existingLB: &loadBalancer{
				shared:              true,
				sslPolicy:           "ELBSecurityPolicy-2016-08",
				sslPolicyIsExplicit: true, // LB was created with an explicit annotation
				ingresses:           make(map[string][]*kubernetes.Ingress),
				scheme:              "internet-facing",
				loadBalancerType:    aws.LoadBalancerTypeApplication,
			},
			incomingIngress: &kubernetes.Ingress{
				SSLPolicy:              "ELBSecurityPolicy-TLS-1-2-2017-01",
				HasSSLPolicyAnnotation: false, // Using default from command line
				Shared:                 true,
				Scheme:                 "internet-facing",
				LoadBalancerType:       aws.LoadBalancerTypeApplication,
			},
			maxCerts:    10,
			shouldAdd:   false,
			description: "Ingress without annotation must NOT share LB whose policy was set explicitly; adding it would overwrite the LB policy and cause an update loop",
		},
		{
			name: "shared LB - default policy on LB - no annotation on ingress - different policy - should share (allows in-place update)",
			existingLB: &loadBalancer{
				shared:              true,
				sslPolicy:           "ELBSecurityPolicy-2016-08",
				sslPolicyIsExplicit: false, // LB was created with the global default
				ingresses:           make(map[string][]*kubernetes.Ingress),
				scheme:              "internet-facing",
				loadBalancerType:    aws.LoadBalancerTypeApplication,
			},
			incomingIngress: &kubernetes.Ingress{
				SSLPolicy:              "ELBSecurityPolicy-TLS-1-2-2017-01",
				HasSSLPolicyAnnotation: false, // Using global default (which changed)
				Shared:                 true,
				Scheme:                 "internet-facing",
				LoadBalancerType:       aws.LoadBalancerTypeApplication,
			},
			maxCerts:    10,
			shouldAdd:   true,
			description: "When both sides use the global default and the default changes, the unannotated ingress should join the existing LB for an in-place update",
		},
		{
			name: "shared LB - no annotation - same policy - should share",
			existingLB: &loadBalancer{
				shared:           true,
				sslPolicy:        "ELBSecurityPolicy-TLS-1-2-2017-01",
				ingresses:        make(map[string][]*kubernetes.Ingress),
				scheme:           "internet-facing",
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
			description: "Ingress without annotation should share LB when SSL policies are identical",
		},
		{
			name: "shared LB - with annotation - matching policy - should share",
			existingLB: &loadBalancer{
				shared:           true,
				sslPolicy:        "ELBSecurityPolicy-TLS-1-2-2017-01",
				ingresses:        make(map[string][]*kubernetes.Ingress),
				scheme:           "internet-facing",
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
				shared:           true,
				sslPolicy:        "ELBSecurityPolicy-2016-08",
				ingresses:        make(map[string][]*kubernetes.Ingress),
				scheme:           "internet-facing",
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
			name: "shared LB - default policy on LB - annotated ingress - different policy - should NOT share",
			existingLB: &loadBalancer{
				shared:              true,
				sslPolicy:           "ELBSecurityPolicy-2016-08",
				sslPolicyIsExplicit: false, // LB was created with the global default
				ingresses:           make(map[string][]*kubernetes.Ingress),
				scheme:              "internet-facing",
				loadBalancerType:    aws.LoadBalancerTypeApplication,
			},
			incomingIngress: &kubernetes.Ingress{
				SSLPolicy:              "ELBSecurityPolicy-TLS-1-2-2017-01",
				HasSSLPolicyAnnotation: true, // Ingress has an explicit annotation
				Shared:                 true,
				Scheme:                 "internet-facing",
				LoadBalancerType:       aws.LoadBalancerTypeApplication,
			},
			maxCerts:    10,
			shouldAdd:   false,
			description: "Annotated ingress must NOT join a default-policy LB when policies differ",
		},
		{
			name: "shared LB - both sides explicit - same policy - should share",
			existingLB: &loadBalancer{
				shared:              true,
				sslPolicy:           "ELBSecurityPolicy-TLS-1-2-2017-01",
				sslPolicyIsExplicit: true,
				ingresses:           make(map[string][]*kubernetes.Ingress),
				scheme:              "internet-facing",
				loadBalancerType:    aws.LoadBalancerTypeApplication,
			},
			incomingIngress: &kubernetes.Ingress{
				SSLPolicy:              "ELBSecurityPolicy-TLS-1-2-2017-01",
				HasSSLPolicyAnnotation: true,
				Shared:                 true,
				Scheme:                 "internet-facing",
				LoadBalancerType:       aws.LoadBalancerTypeApplication,
			},
			maxCerts:    10,
			shouldAdd:   true,
			description: "Two ingresses with identical explicit SSL policy annotations should share the LB",
		},
		{
			name: "shared LB - both sides explicit - different policy - should NOT share",
			existingLB: &loadBalancer{
				shared:              true,
				sslPolicy:           "ELBSecurityPolicy-2016-08",
				sslPolicyIsExplicit: true,
				ingresses:           make(map[string][]*kubernetes.Ingress),
				scheme:              "internet-facing",
				loadBalancerType:    aws.LoadBalancerTypeApplication,
			},
			incomingIngress: &kubernetes.Ingress{
				SSLPolicy:              "ELBSecurityPolicy-TLS-1-2-2017-01",
				HasSSLPolicyAnnotation: true,
				Shared:                 true,
				Scheme:                 "internet-facing",
				LoadBalancerType:       aws.LoadBalancerTypeApplication,
			},
			maxCerts:    10,
			shouldAdd:   false,
			description: "Two ingresses with different explicit SSL policy annotations must NOT share the LB",
		},
		{
			name: "shared LB - both sides default - same policy - should share",
			existingLB: &loadBalancer{
				shared:              true,
				sslPolicy:           "ELBSecurityPolicy-TLS-1-2-2017-01",
				sslPolicyIsExplicit: false,
				ingresses:           make(map[string][]*kubernetes.Ingress),
				scheme:              "internet-facing",
				loadBalancerType:    aws.LoadBalancerTypeApplication,
			},
			incomingIngress: &kubernetes.Ingress{
				SSLPolicy:              "ELBSecurityPolicy-TLS-1-2-2017-01",
				HasSSLPolicyAnnotation: false,
				Shared:                 true,
				Scheme:                 "internet-facing",
				LoadBalancerType:       aws.LoadBalancerTypeApplication,
			},
			maxCerts:    10,
			shouldAdd:   true,
			description: "Two unannotated ingresses with the same global default policy should share the LB",
		},
		{
			name: "non-shared LB - always allows different SSL policies",
			existingLB: &loadBalancer{
				shared:           false,
				sslPolicy:        "ELBSecurityPolicy-2016-08",
				ingresses:        make(map[string][]*kubernetes.Ingress),
				scheme:           "internet-facing",
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
