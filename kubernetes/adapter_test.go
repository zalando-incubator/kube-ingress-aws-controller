package kubernetes

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
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
	testLoadBalancerTypeAWS         = aws.LoadBalancerTypeApplication
	testWAFWebACLId                 = "zbr-1234"
)

func TestMappingRoundtrip(tt *testing.T) {
	for _, tc := range []struct {
		msg         string
		ingress     *Ingress
		kubeIngress *ingress
	}{
		{
			msg: "test parsing a simple ingress object",
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
				LoadBalancerType: testLoadBalancerTypeAWS,
				resourceType:     ingressTypeIngress,
				WAFWebACLId:      testWAFWebACLId,
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
						ingressWAFWebACLIdAnnotation:      testWAFWebACLId,
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
			msg: "test parsing an ingress object with cluster.local domain",
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
				LoadBalancerType: testLoadBalancerTypeAWS,
				resourceType:     ingressTypeIngress,
				WAFWebACLId:      testWAFWebACLId,
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
						ingressWAFWebACLIdAnnotation:      testWAFWebACLId,
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
			msg: "test parsing an ingress object with shared=false,h2-enabled=false annotations",
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
				LoadBalancerType: testLoadBalancerTypeAWS,
				resourceType:     ingressTypeIngress,
				WAFWebACLId:      testWAFWebACLId,
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
						ingressWAFWebACLIdAnnotation:      testWAFWebACLId,
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
			msg: "test parsing an ingress object with dualstack annotation",
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
				LoadBalancerType: testLoadBalancerTypeAWS,
				resourceType:     ingressTypeIngress,
				WAFWebACLId:      testWAFWebACLId,
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
						ingressWAFWebACLIdAnnotation:      testWAFWebACLId,
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
	} {
		tt.Run(tc.msg, func(t *testing.T) {
			a, err := NewAdapter(testConfig, testIngressFilter, testIngressDefaultSecurityGroup, testSSLPolicy, testLoadBalancerTypeAWS, DefaultClusterLocalDomain)
			if err != nil {
				t.Fatalf("cannot create kubernetes adapter: %v", err)
			}

			got := a.newIngressFromKube(tc.kubeIngress)
			assert.Equal(t, tc.ingress, got, "mapping from kubernetes ingress to adapter failed")
			assert.Equal(t, got.String(), fmt.Sprintf("%s/%s", tc.ingress.Namespace, tc.ingress.Name), "wrong value from String()")

			tc.kubeIngress.Status.LoadBalancer.Ingress = tc.kubeIngress.Status.LoadBalancer.Ingress[1:]
			gotKube := newIngressForKube(got)
			assert.Equal(t, tc.kubeIngress.Metadata, gotKube.Metadata, "mapping from adapter to kubernetes ingress failed")
			assert.Equal(t, tc.kubeIngress.Status, gotKube.Status, "mapping from adapter to kubernetes ingress failed")
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
	case routegroupListResource:
		fixture = "testdata/fixture01_rg.json"
	case ingressListResource:
		fixture = "testdata/fixture01.json"
	case fmt.Sprintf(configMapResource, "foo-ns", "foo-name"):
		fixture = "testdata/fixture02.json"
	default:
		return nil, fmt.Errorf("unexpected resource: %s", res)
	}

	buf, err := ioutil.ReadFile(fixture)
	if err != nil {
		return nil, err
	}
	return ioutil.NopCloser(bytes.NewReader(buf)), nil
}

func (c *mockClient) patch(res string, payload []byte) (io.ReadCloser, error) {
	if !c.broken {
		switch res {
		case "/apis/extensions/v1beta1/namespaces/default/ingresses/foo/status":
			return ioutil.NopCloser(strings.NewReader(":)")), nil
		case "/apis/zalando.org/v1/namespaces/default/routegroups/foo/status":
			return ioutil.NopCloser(strings.NewReader(":)")), nil
		}
	}
	return nil, errors.New("mocked error")
}

func TestListIngress(t *testing.T) {
	a, _ := NewAdapter(testConfig, testIngressFilter, testIngressDefaultSecurityGroup, testSSLPolicy, testLoadBalancerTypeAWS, DefaultClusterLocalDomain)
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

func TestUpdateIngressLoadBalancer(t *testing.T) {
	a, _ := NewAdapter(testConfig, testIngressFilter, testSecurityGroup, testSSLPolicy, testLoadBalancerTypeAWS, DefaultClusterLocalDomain)
	client := &mockClient{}
	a.kubeClient = client
	ing := &Ingress{
		Namespace:      "default",
		Name:           "foo",
		Hostname:       "bar",
		CertificateARN: "zbr",
		resourceType:   ingressTypeIngress,
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
	a, _ := NewAdapter(testConfig, testIngressFilter, testSecurityGroup, testSSLPolicy, testLoadBalancerTypeAWS, DefaultClusterLocalDomain)
	client := &mockClient{}
	a.kubeClient = client
	ing := &Ingress{
		Namespace:      "default",
		Name:           "foo",
		Hostname:       "bar",
		CertificateARN: "zbr",
		resourceType:   ingressTypeRouteGroup,
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
			_, err := NewAdapter(test.cfg, testIngressFilter, testSecurityGroup, testSSLPolicy, testLoadBalancerTypeAWS, DefaultClusterLocalDomain)
			if err == nil {
				t.Error("expected an error")
			}
		})
	}
}

func TestAdapter_GetConfigMap(t *testing.T) {
	a, _ := NewAdapter(testConfig, testIngressFilter, testIngressDefaultSecurityGroup, testSSLPolicy, testLoadBalancerTypeAWS, DefaultClusterLocalDomain)
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
				"fixture-rg01",
				"fixture-rg02",
				"fixture-rg03",
			},
		},
		"emptyIngressClassFilters2": {
			ingressClassFilters: []string{},
			expectedIngressNames: []string{
				"fixture01",
				"fixture02",
				"fixture03",
				"fixture-rg01",
				"fixture-rg02",
				"fixture-rg03",
			},
		},
		"singleIngressClass1": {
			ingressClassFilters: []string{"skipper"},
			expectedIngressNames: []string{
				"fixture02",
				"fixture-rg02",
			},
		},
		"singleIngressClass2": {
			ingressClassFilters: []string{"other"},
			expectedIngressNames: []string{
				"fixture03",
				"fixture-rg03",
			},
		},
		"multipleIngressClass": {
			ingressClassFilters: []string{"skipper", "other"},
			expectedIngressNames: []string{
				"fixture02",
				"fixture03",
				"fixture-rg02",
				"fixture-rg03",
			},
		},
		"multipleIngressClassWithDefault": {
			ingressClassFilters: []string{"skipper", ""},
			expectedIngressNames: []string{
				"fixture01",
				"fixture02",
				"fixture-rg01",
				"fixture-rg02",
			},
		},
		"multipleIngressClassWithDefault2": {
			ingressClassFilters: []string{"other", ""},
			expectedIngressNames: []string{
				"fixture01",
				"fixture03",
				"fixture-rg01",
				"fixture-rg03",
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			a, _ := NewAdapter(testConfig, test.ingressClassFilters, testIngressDefaultSecurityGroup, testSSLPolicy, testLoadBalancerTypeAWS, DefaultClusterLocalDomain)
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
