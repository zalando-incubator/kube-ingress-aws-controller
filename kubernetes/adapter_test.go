package kubernetes

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zalando-incubator/kube-ingress-aws-controller/aws"
)

var (
	testConfig                      = InsecureConfig("dummy-url")
	testIngressFilter               = []string{"skipper"}
	testIngressDefaultSecurityGroup = "sg-foobar"
	testSecurityGroup               = "sg-123456"
	testSSLPolicy                   = "ELBSecurityPolicy-TLS-1-2-2017-01"
	testIPAddressTypeDualStack      = aws.IPAddressTypeDualstack
	testIPAddressTypeDefault        = aws.IPAddressTypeIPV4
	testLoadBalancerTypeIngress     = loadBalancerTypeALB
	testWAFWebACLID                 = "zbr-1234"
)

func TestNewIngressFromKube(tt *testing.T) {
	for _, tc := range []struct {
		msg                     string
		defaultLoadBalancerType string
		ingress                 *Ingress
		ingressError            bool
		kubeIngress             *ingress
	}{
		{
			msg:                     "test parsing a simple ingress object",
			defaultLoadBalancerType: aws.LoadBalancerTypeApplication,
			ingress: &Ingress{
				Namespace:        "default",
				Name:             "foo",
				Hostname:         "bar",
				Scheme:           "internal",
				CertificateARN:   "zbr",
				Shared:           true,
				HTTP2:            true,
				Hostnames:        []string{"domain.example.org"},
				SecurityGroup:    testSecurityGroup,
				SSLPolicy:        testSSLPolicy,
				IPAddressType:    testIPAddressTypeDefault,
				LoadBalancerType: aws.LoadBalancerTypeApplication,
				ResourceType:     TypeIngress,
				WAFWebACLID:      testWAFWebACLID,
			},
			kubeIngress: &ingress{
				Metadata: kubeItemMetadata{
					Namespace: "default",
					Name:      "foo",
					Annotations: map[string]string{
						ingressCertificateARNAnnotation:   "zbr",
						ingressSchemeAnnotation:           "internal",
						ingressSharedAnnotation:           "true",
						ingressHTTP2Annotation:            "true",
						ingressSecurityGroupAnnotation:    testSecurityGroup,
						ingressSSLPolicyAnnotation:        testSSLPolicy,
						ingressALBIPAddressType:           testIPAddressTypeDefault,
						ingressLoadBalancerTypeAnnotation: testLoadBalancerTypeIngress,
						ingressWAFWebACLIDAnnotation:      testWAFWebACLID,
					},
				},
				Spec: ingressSpec{
					Rules: []ingressItemRule{
						{
							Host: "domain.example.org",
						},
					},
				},
				Status: ingressStatus{
					LoadBalancer: ingressLoadBalancerStatus{
						Ingress: []ingressLoadBalancer{
							{Hostname: ""},
							{Hostname: "bar"},
						},
					},
				},
			},
		},
		{
			msg:                     "test parsing an ingress object with cluster.local domain",
			defaultLoadBalancerType: aws.LoadBalancerTypeApplication,
			ingress: &Ingress{
				Namespace:        "default",
				Name:             "foo",
				Hostname:         "bar",
				Scheme:           "internal",
				CertificateARN:   "zbr",
				Shared:           true,
				HTTP2:            true,
				ClusterLocal:     true,
				SecurityGroup:    testSecurityGroup,
				SSLPolicy:        testSSLPolicy,
				IPAddressType:    testIPAddressTypeDefault,
				LoadBalancerType: aws.LoadBalancerTypeApplication,
				ResourceType:     TypeIngress,
				WAFWebACLID:      testWAFWebACLID,
			},
			kubeIngress: &ingress{
				Metadata: kubeItemMetadata{
					Namespace: "default",
					Name:      "foo",
					Annotations: map[string]string{
						ingressCertificateARNAnnotation:   "zbr",
						ingressSchemeAnnotation:           "internal",
						ingressSharedAnnotation:           "true",
						ingressHTTP2Annotation:            "true",
						ingressSecurityGroupAnnotation:    testSecurityGroup,
						ingressSSLPolicyAnnotation:        testSSLPolicy,
						ingressALBIPAddressType:           testIPAddressTypeDefault,
						ingressLoadBalancerTypeAnnotation: testLoadBalancerTypeIngress,
						ingressWAFWebACLIDAnnotation:      testWAFWebACLID,
					},
				},
				Spec: ingressSpec{
					Rules: []ingressItemRule{
						{
							Host: "domain.cluster.local",
						},
					},
				},
				Status: ingressStatus{
					LoadBalancer: ingressLoadBalancerStatus{
						Ingress: []ingressLoadBalancer{
							{Hostname: ""},
							{Hostname: "bar"},
						},
					},
				},
			},
		},
		{
			msg:                     "test parsing an ingress object with shared=false,h2-enabled=false annotations",
			defaultLoadBalancerType: aws.LoadBalancerTypeApplication,
			ingress: &Ingress{
				Namespace:        "default",
				Name:             "foo",
				Hostname:         "bar",
				Scheme:           "internal",
				CertificateARN:   "zbr",
				Shared:           false,
				HTTP2:            false,
				ClusterLocal:     true,
				SecurityGroup:    testSecurityGroup,
				SSLPolicy:        testSSLPolicy,
				IPAddressType:    testIPAddressTypeDefault,
				LoadBalancerType: aws.LoadBalancerTypeApplication,
				ResourceType:     TypeIngress,
				WAFWebACLID:      testWAFWebACLID,
			},
			kubeIngress: &ingress{
				Metadata: kubeItemMetadata{
					Namespace: "default",
					Name:      "foo",
					Annotations: map[string]string{
						ingressCertificateARNAnnotation:   "zbr",
						ingressSchemeAnnotation:           "internal",
						ingressSharedAnnotation:           "false",
						ingressHTTP2Annotation:            "false",
						ingressSecurityGroupAnnotation:    testSecurityGroup,
						ingressSSLPolicyAnnotation:        testSSLPolicy,
						ingressALBIPAddressType:           testIPAddressTypeDefault,
						ingressLoadBalancerTypeAnnotation: testLoadBalancerTypeIngress,
						ingressWAFWebACLIDAnnotation:      testWAFWebACLID,
					},
				},
				Status: ingressStatus{
					LoadBalancer: ingressLoadBalancerStatus{
						Ingress: []ingressLoadBalancer{
							{Hostname: ""},
							{Hostname: "bar"},
						},
					},
				},
			},
		},
		{
			msg:                     "test parsing an ingress object with dualstack annotation",
			defaultLoadBalancerType: aws.LoadBalancerTypeApplication,
			ingress: &Ingress{
				Namespace:        "default",
				Name:             "foo",
				Hostname:         "bar",
				Scheme:           "internal",
				CertificateARN:   "zbr",
				Shared:           true,
				HTTP2:            true,
				ClusterLocal:     true,
				SecurityGroup:    testSecurityGroup,
				SSLPolicy:        testSSLPolicy,
				IPAddressType:    testIPAddressTypeDualStack,
				LoadBalancerType: aws.LoadBalancerTypeApplication,
				ResourceType:     TypeIngress,
				WAFWebACLID:      testWAFWebACLID,
			},
			kubeIngress: &ingress{
				Metadata: kubeItemMetadata{
					Namespace: "default",
					Name:      "foo",
					Annotations: map[string]string{
						ingressCertificateARNAnnotation:   "zbr",
						ingressSchemeAnnotation:           "internal",
						ingressSharedAnnotation:           "true",
						ingressHTTP2Annotation:            "true",
						ingressSecurityGroupAnnotation:    testSecurityGroup,
						ingressSSLPolicyAnnotation:        testSSLPolicy,
						ingressALBIPAddressType:           testIPAddressTypeDualStack,
						ingressLoadBalancerTypeAnnotation: testLoadBalancerTypeIngress,
						ingressWAFWebACLIDAnnotation:      testWAFWebACLID,
					},
				},
				Status: ingressStatus{
					LoadBalancer: ingressLoadBalancerStatus{
						Ingress: []ingressLoadBalancer{
							{Hostname: ""},
							{Hostname: "bar"},
						},
					},
				},
			},
		},
		{
			msg:                     "test default NLB without annotations",
			defaultLoadBalancerType: aws.LoadBalancerTypeNetwork,
			ingress: &Ingress{
				ResourceType:     TypeIngress,
				Namespace:        "default",
				Name:             "foo",
				Hostname:         "bar",
				Scheme:           "internet-facing",
				Shared:           true,
				HTTP2:            true,
				ClusterLocal:     true,
				SSLPolicy:        testSSLPolicy,
				IPAddressType:    aws.IPAddressTypeIPV4,
				LoadBalancerType: aws.LoadBalancerTypeNetwork,
				SecurityGroup:    testIngressDefaultSecurityGroup,
			},
			kubeIngress: &ingress{
				Metadata: kubeItemMetadata{
					Namespace: "default",
					Name:      "foo",
				},
				Status: ingressStatus{
					LoadBalancer: ingressLoadBalancerStatus{
						Ingress: []ingressLoadBalancer{
							{Hostname: "bar"},
						},
					},
				},
			},
		},
		{
			msg:                     "test default NLB with security group fallbacks to ALB",
			defaultLoadBalancerType: aws.LoadBalancerTypeNetwork,
			ingress: &Ingress{
				ResourceType:     TypeIngress,
				Namespace:        "default",
				Name:             "foo",
				Hostname:         "bar",
				Scheme:           "internet-facing",
				Shared:           true,
				HTTP2:            true,
				ClusterLocal:     true,
				SSLPolicy:        testSSLPolicy,
				IPAddressType:    aws.IPAddressTypeIPV4,
				LoadBalancerType: aws.LoadBalancerTypeApplication,
				SecurityGroup:    "sg-custom",
			},
			kubeIngress: &ingress{
				Metadata: kubeItemMetadata{
					Namespace: "default",
					Name:      "foo",
					Annotations: map[string]string{
						ingressSecurityGroupAnnotation: "sg-custom",
					},
				},
				Status: ingressStatus{
					LoadBalancer: ingressLoadBalancerStatus{
						Ingress: []ingressLoadBalancer{
							{Hostname: "bar"},
						},
					},
				},
			},
		},
		{
			msg:                     "test default NLB with internal annotations fallbacks to ALB",
			defaultLoadBalancerType: aws.LoadBalancerTypeNetwork,
			ingress: &Ingress{
				ResourceType:     TypeIngress,
				Namespace:        "default",
				Name:             "foo",
				Hostname:         "bar",
				Scheme:           "internal",
				Shared:           true,
				HTTP2:            true,
				ClusterLocal:     true,
				SSLPolicy:        testSSLPolicy,
				IPAddressType:    aws.IPAddressTypeIPV4,
				LoadBalancerType: aws.LoadBalancerTypeApplication,
				SecurityGroup:    testIngressDefaultSecurityGroup,
			},
			kubeIngress: &ingress{
				Metadata: kubeItemMetadata{
					Namespace: "default",
					Name:      "foo",
					Annotations: map[string]string{
						ingressSchemeAnnotation: "internal",
					},
				},
				Status: ingressStatus{
					LoadBalancer: ingressLoadBalancerStatus{
						Ingress: []ingressLoadBalancer{
							{Hostname: "bar"},
						},
					},
				},
			},
		},
		{
			msg:                     "test default ALB with lb type annotation nlb and internal annotation uses NLB",
			defaultLoadBalancerType: aws.LoadBalancerTypeApplication,
			ingress: &Ingress{
				ResourceType:     TypeIngress,
				Namespace:        "default",
				Name:             "foo",
				Hostname:         "bar",
				Scheme:           "internal",
				Shared:           true,
				HTTP2:            true,
				ClusterLocal:     true,
				SSLPolicy:        testSSLPolicy,
				IPAddressType:    aws.IPAddressTypeIPV4,
				LoadBalancerType: aws.LoadBalancerTypeNetwork,
				SecurityGroup:    testIngressDefaultSecurityGroup,
			},
			kubeIngress: &ingress{
				Metadata: kubeItemMetadata{
					Namespace: "default",
					Name:      "foo",
					Annotations: map[string]string{
						ingressSchemeAnnotation:           "internal",
						ingressLoadBalancerTypeAnnotation: "nlb",
					},
				},
				Status: ingressStatus{
					LoadBalancer: ingressLoadBalancerStatus{
						Ingress: []ingressLoadBalancer{
							{Hostname: "bar"},
						},
					},
				},
			},
		},
		{
			msg:                     "test default NLB with WAF fallbacks to ALB",
			defaultLoadBalancerType: aws.LoadBalancerTypeNetwork,
			ingress: &Ingress{
				ResourceType:     TypeIngress,
				Namespace:        "default",
				Name:             "foo",
				Hostname:         "bar",
				Scheme:           "internet-facing",
				Shared:           true,
				HTTP2:            true,
				ClusterLocal:     true,
				SSLPolicy:        testSSLPolicy,
				IPAddressType:    aws.IPAddressTypeIPV4,
				LoadBalancerType: aws.LoadBalancerTypeApplication,
				SecurityGroup:    testIngressDefaultSecurityGroup,
				WAFWebACLID:      "waf-custom",
			},
			kubeIngress: &ingress{
				Metadata: kubeItemMetadata{
					Namespace: "default",
					Name:      "foo",
					Annotations: map[string]string{
						ingressWAFWebACLIDAnnotation: "waf-custom",
					},
				},
				Status: ingressStatus{
					LoadBalancer: ingressLoadBalancerStatus{
						Ingress: []ingressLoadBalancer{
							{Hostname: "bar"},
						},
					},
				},
			},
		},
		{
			msg:                     "test explicitly configured NLB with security group raises error",
			defaultLoadBalancerType: aws.LoadBalancerTypeApplication,
			ingressError:            true,
			kubeIngress: &ingress{
				Metadata: kubeItemMetadata{
					Namespace: "default",
					Name:      "foo",
					Annotations: map[string]string{
						ingressLoadBalancerTypeAnnotation: loadBalancerTypeNLB,
						ingressSecurityGroupAnnotation:    "sg-custom",
					},
				},
				Status: ingressStatus{
					LoadBalancer: ingressLoadBalancerStatus{
						Ingress: []ingressLoadBalancer{
							{Hostname: "bar"},
						},
					},
				},
			},
		},
		{
			msg:                     "test explicitly configured NLB with WAF raises error",
			defaultLoadBalancerType: aws.LoadBalancerTypeApplication,
			ingressError:            true,
			kubeIngress: &ingress{
				Metadata: kubeItemMetadata{
					Namespace: "default",
					Name:      "foo",
					Annotations: map[string]string{
						ingressLoadBalancerTypeAnnotation: loadBalancerTypeNLB,
						ingressWAFWebACLIDAnnotation:      "waf-custom",
					},
				},
				Status: ingressStatus{
					LoadBalancer: ingressLoadBalancerStatus{
						Ingress: []ingressLoadBalancer{
							{Hostname: "bar"},
						},
					},
				},
			},
		},
	} {
		tt.Run(tc.msg, func(t *testing.T) {
			a, err := NewAdapter(testConfig, IngressAPIVersionNetworking, testIngressFilter, testIngressDefaultSecurityGroup, testSSLPolicy, tc.defaultLoadBalancerType, DefaultClusterLocalDomain, false)
			if err != nil {
				t.Fatalf("cannot create kubernetes adapter: %v", err)
			}

			got, err := a.newIngressFromKube(tc.kubeIngress)
			if tc.ingressError {
				assert.NotNil(t, err)
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.ingress, got, "mapping from kubernetes ingress to adapter failed")
				assert.Equal(t, got.String(), fmt.Sprintf("%s %s/%s", tc.ingress.ResourceType, tc.ingress.Namespace, tc.ingress.Name), "wrong value from String()")
			}
		})
	}
}

func TestInsecureConfig(t *testing.T) {
	cfg := InsecureConfig("http://domain.com:12345")
	if cfg.BaseURL != "http://domain.com:12345" {
		t.Errorf(`unexpected base URL. wanted "http://domain.com:12345", got %q`, cfg.BaseURL)
	}
	if cfg.CAFile != "" {
		t.Error("unexpected CAFile attribute")
	}
	if cfg.BearerToken != "" {
		t.Error("unexpected Bearer token attribute")
	}
	if cfg.UserAgent != defaultControllerUserAgent {
		t.Error("unexpected User Agent attribute")
	}
}

type mockClient struct {
	broken bool
}

func (c *mockClient) get(res string) (io.ReadCloser, error) {
	if c.broken {
		return nil, errors.New("mocked error")
	}

	var fixture string
	switch res {
	case fabricListResource:
		fixture = "testdata/fixture01_fg.json"
	case routegroupListResource:
		fixture = "testdata/fixture01_rg.json"
	case fmt.Sprintf(ingressListResource, IngressAPIVersionNetworking):
		fixture = "testdata/fixture01.json"
	case fmt.Sprintf(configMapResource, "foo-ns", "foo-name"):
		fixture = "testdata/fixture02.json"
	default:
		return nil, fmt.Errorf("unexpected resource: %s", res)
	}

	buf, err := os.ReadFile(fixture)
	if err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(buf)), nil
}

func (c *mockClient) patch(res string, payload []byte) (io.ReadCloser, error) {
	if !c.broken {
		switch res {
		case fmt.Sprintf("/apis/%s/namespaces/default/ingresses/foo/status", IngressAPIVersionNetworking):
			return io.NopCloser(strings.NewReader(":)")), nil
		case "/apis/zalando.org/v1/namespaces/default/routegroups/foo/status":
			return io.NopCloser(strings.NewReader(":)")), nil
		case "/apis/zalando.org/v1/namespaces/default/fabricgateways/foo/status":
			return io.NopCloser(strings.NewReader(":)")), nil
		}
	}
	return nil, errors.New("mocked error")
}

func (c *mockClient) post(res string, payload []byte) (io.ReadCloser, error) {
	return nil, errors.New("not implemented")
}

func TestListIngress(t *testing.T) {
	a, _ := NewAdapter(testConfig, IngressAPIVersionNetworking, testIngressFilter, testIngressDefaultSecurityGroup, testSSLPolicy, aws.LoadBalancerTypeApplication, DefaultClusterLocalDomain, false)
	client := &mockClient{}
	a.kubeClient = client
	ingresses, err := a.ListIngress()
	if err != nil {
		t.Error(err)
	}
	if len(ingresses) != 1 {
		t.Fatal("unexpected count of ingress resources")
	}
	client.broken = true
	_, err = a.ListIngress()
	if err == nil {
		t.Error("expected an error")
	}
}

func TestAdapterUpdateIngressLoadBalancer(t *testing.T) {
	a, _ := NewAdapter(testConfig, IngressAPIVersionNetworking, testIngressFilter, testSecurityGroup, testSSLPolicy, aws.LoadBalancerTypeApplication, DefaultClusterLocalDomain, false)
	client := &mockClient{}
	a.kubeClient = client
	ing := &Ingress{
		Namespace:      "default",
		Name:           "foo",
		Hostname:       "bar",
		CertificateARN: "zbr",
		ResourceType:   TypeIngress,
	}
	if err := a.UpdateIngressLoadBalancer(ing, "bar"); err != ErrUpdateNotNeeded {
		t.Error("expected ErrUpdateNotNeeded")
	}
	if err := a.UpdateIngressLoadBalancer(ing, "xpto"); err != nil {
		t.Error(err)
	}
	client.broken = true
	if err := a.UpdateIngressLoadBalancer(ing, "xpto"); err == nil {
		t.Error("expected an error")
	}
	if err := a.UpdateIngressLoadBalancer(ing, ""); err == nil {
		t.Error("expected an error")
	}
	if err := a.UpdateIngressLoadBalancer(nil, "xpto"); err == nil {
		t.Error("expected an error")
	}
}

func TestUpdateRouteGroupLoadBalancer(t *testing.T) {
	a, _ := NewAdapter(testConfig, IngressAPIVersionNetworking, testIngressFilter, testSecurityGroup, testSSLPolicy, aws.LoadBalancerTypeApplication, DefaultClusterLocalDomain, false)
	client := &mockClient{}
	a.kubeClient = client
	ing := &Ingress{
		Namespace:      "default",
		Name:           "foo",
		Hostname:       "bar",
		CertificateARN: "zbr",
		ResourceType:   TypeRouteGroup,
	}
	if err := a.UpdateIngressLoadBalancer(ing, "bar"); err != ErrUpdateNotNeeded {
		t.Error("expected ErrUpdateNotNeeded")
	}
	if err := a.UpdateIngressLoadBalancer(ing, "xpto"); err != nil {
		t.Error(err)
	}
	client.broken = true
	if err := a.UpdateIngressLoadBalancer(ing, "xpto"); err == nil {
		t.Error("expected an error")
	}
	if err := a.UpdateIngressLoadBalancer(ing, ""); err == nil {
		t.Error("expected an error")
	}
	if err := a.UpdateIngressLoadBalancer(nil, "xpto"); err == nil {
		t.Error("expected an error")
	}
}

func TestUpdateFabricGatewayLoadBalancer(t *testing.T) {
	a, _ := NewAdapter(testConfig, IngressAPIVersionNetworking, testIngressFilter, testSecurityGroup, testSSLPolicy, aws.LoadBalancerTypeApplication, DefaultClusterLocalDomain, false)
	client := &mockClient{}
	a.kubeClient = client
	ing := &Ingress{
		Namespace:      "default",
		Name:           "foo",
		Hostname:       "bar",
		CertificateARN: "zbr",
		ResourceType:   TypeFabricGateway,
	}
	if err := a.UpdateIngressLoadBalancer(ing, "bar"); err != ErrUpdateNotNeeded {
		t.Error("expected ErrUpdateNotNeeded")
	}
	if err := a.UpdateIngressLoadBalancer(ing, "xpto"); err != nil {
		t.Error(err)
	}
	client.broken = true
	if err := a.UpdateIngressLoadBalancer(ing, "xpto"); err == nil {
		t.Error("expected an error")
	}
	if err := a.UpdateIngressLoadBalancer(ing, ""); err == nil {
		t.Error("expected an error")
	}
	if err := a.UpdateIngressLoadBalancer(nil, "xpto"); err == nil {
		t.Error("expected an error")
	}
}

func TestBrokenConfig(t *testing.T) {
	for _, test := range []struct {
		name string
		cfg  *Config
	}{
		{"nil-configuraion", nil},
		{"empty-configuration", &Config{}},
		{"missing-cert", &Config{BaseURL: "dontcare", TLSClientConfig: TLSClientConfig{CAFile: "missing"}}},
		{"broken-cert", &Config{BaseURL: "dontcare", TLSClientConfig: TLSClientConfig{CAFile: "testdata/broken.pem"}}},
	} {
		t.Run(fmt.Sprintf("%v", test.cfg), func(t *testing.T) {
			_, err := NewAdapter(test.cfg, IngressAPIVersionNetworking, testIngressFilter, testSecurityGroup, testSSLPolicy, aws.LoadBalancerTypeApplication, DefaultClusterLocalDomain, false)
			if err == nil {
				t.Error("expected an error")
			}
		})
	}
}

func TestAdapter_GetConfigMap(t *testing.T) {
	a, _ := NewAdapter(testConfig, IngressAPIVersionNetworking, testIngressFilter, testIngressDefaultSecurityGroup, testSSLPolicy, aws.LoadBalancerTypeApplication, DefaultClusterLocalDomain, false)
	client := &mockClient{}
	a.kubeClient = client

	cm, err := a.GetConfigMap("foo-ns", "foo-name")
	if err != nil {
		t.Error(err)
	}

	expectedData := map[string]string{"some-key": "key1: val1\nkey2: val2\n"}

	if !reflect.DeepEqual(cm.Data, expectedData) {
		t.Fatalf("unexpected ConfigMap data, got %+v, want %+v", cm.Data, expectedData)
	}

	client.broken = true
	_, err = a.GetConfigMap("foo-ns", "foo-name")
	if err == nil {
		t.Error("expected an error")
	}
}

func TestListIngressFilterClass(t *testing.T) {
	for name, test := range map[string]struct {
		ingressClassFilters  []string
		expectedIngressNames []string
	}{
		"emptyIngressClassFilters": {
			ingressClassFilters: nil,
			expectedIngressNames: []string{
				"fixture01",
				"fixture02",
				"fixture03",
				"fixture04",
				"fixture05",
				"fixture-rg01",
				"fixture-rg02",
				"fixture-rg03",
				"fixture-fg01",
				"fixture-fg02",
				"fixture-fg03",
			},
		},
		"emptyIngressClassFilters2": {
			ingressClassFilters: []string{},
			expectedIngressNames: []string{
				"fixture01",
				"fixture02",
				"fixture03",
				"fixture04",
				"fixture05",
				"fixture-rg01",
				"fixture-rg02",
				"fixture-rg03",
				"fixture-fg01",
				"fixture-fg02",
				"fixture-fg03",
			},
		},
		"singleIngressClass1": {
			ingressClassFilters: []string{"skipper"},
			expectedIngressNames: []string{
				"fixture02",
				"fixture-rg02",
				"fixture-fg02",
			},
		},
		"singleIngressClass2": {
			ingressClassFilters: []string{"other"},
			expectedIngressNames: []string{
				"fixture03",
				"fixture-rg03",
				"fixture-fg03",
			},
		},
		"multipleIngressClass": {
			ingressClassFilters: []string{"skipper", "other"},
			expectedIngressNames: []string{
				"fixture02",
				"fixture03",
				"fixture-rg02",
				"fixture-rg03",
				"fixture-fg02",
				"fixture-fg03",
			},
		},
		"multipleIngressClassWithDefault": {
			ingressClassFilters: []string{"skipper", ""},
			expectedIngressNames: []string{
				"fixture01",
				"fixture02",
				"fixture-rg01",
				"fixture-rg02",
				"fixture-fg01",
				"fixture-fg02",
			},
		},
		"multipleIngressClassWithDefault2": {
			ingressClassFilters: []string{"other", ""},
			expectedIngressNames: []string{
				"fixture01",
				"fixture03",
				"fixture-rg01",
				"fixture-rg03",
				"fixture-fg01",
				"fixture-fg03",
			},
		},
		"multipleIngressClassMixedAnnotationAndSpec": {
			ingressClassFilters: []string{"other", "another"},
			expectedIngressNames: []string{
				"fixture03",
				"fixture-rg03",
				"fixture-fg03",
				"fixture04",
			},
		},
		"multipleIngressClassOnlySpec": {
			ingressClassFilters: []string{"another"},
			expectedIngressNames: []string{
				"fixture04",
			},
		},
		"multipleIngressIgnoreAnnotationIfClassIsSet": {
			ingressClassFilters:  []string{"yet-another-ignored"},
			expectedIngressNames: []string{},
		},
	} {
		t.Run(name, func(t *testing.T) {
			a, _ := NewAdapter(testConfig, IngressAPIVersionNetworking, test.ingressClassFilters, testIngressDefaultSecurityGroup, testSSLPolicy, aws.LoadBalancerTypeApplication, DefaultClusterLocalDomain, false)
			client := &mockClient{}
			a.kubeClient = client
			ingresses, err := a.ListResources()
			if err != nil {
				t.Error(err)
			}
			ingressNames := make([]string, len(ingresses))
			for i, ing := range ingresses {
				ingressNames[i] = ing.Name
			}
			assert.ElementsMatch(t, test.expectedIngressNames, ingressNames, "ingress names mismatch")
		})
	}
}

func TestWithTargetCNIPodSelector(t *testing.T) {
	t.Run("WithTargetCNIPodSelector sets the targetPort property", func(t *testing.T) {
		a := &Adapter{}
		a = a.WithTargetCNIPodSelector("kube-system", "application=skipper-ingress")
		assert.Equal(t, "kube-system", a.cniPodNamespace)
		assert.Equal(t, "application=skipper-ingress", a.cniPodLabelSelector)
	})
}

func TestConfigureFabricSSLPolicy(t *testing.T) {
	require.Contains(t, aws.SSLPolicies, defaultFabricSSLPolicy)

	for _, test := range []struct {
		name     string
		fg       *fabric
		expected string
	}{
		{
			name:     "no annotations",
			fg:       &fabric{},
			expected: defaultFabricSSLPolicy,
		},
		{
			name:     "other annotations",
			fg:       &fabric{Metadata: kubeItemMetadata{Annotations: map[string]string{"foo": "bar"}}},
			expected: defaultFabricSSLPolicy,
		},
		{
			name:     "allow override",
			fg:       &fabric{Metadata: kubeItemMetadata{Annotations: map[string]string{ingressSSLPolicyAnnotation: "a-policy"}}},
			expected: "a-policy",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			configureFabricSSLPolicy(test.fg)
			assert.Equal(t, test.expected, test.fg.Metadata.Annotations[ingressSSLPolicyAnnotation])
		})
	}
}
