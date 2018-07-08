package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zalando-incubator/kube-ingress-aws-controller/aws"
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
