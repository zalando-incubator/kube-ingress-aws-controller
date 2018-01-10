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
)

func TestMappingRoundtrip(t *testing.T) {
	i := &Ingress{
		namespace:      "default",
		name:           "foo",
		hostName:       "bar",
		scheme:         "internal",
		certificateARN: "zbr",
	}

	kubeMeta := ingressItemMetadata{
		Namespace: "default",
		Name:      "foo",
		Annotations: map[string]interface{}{
			ingressCertificateARNAnnotation: "zbr",
			ingressSchemeAnnotation:         "internal",
		},
	}
	kubeStatus := ingressStatus{
		LoadBalancer: ingressLoadBalancerStatus{
			Ingress: []ingressLoadBalancer{
				{Hostname: ""},
				{Hostname: "bar"},
			},
		},
	}
	kubeIngress := &ingress{
		Metadata: kubeMeta,
		Status:   kubeStatus,
	}

	got := newIngressFromKube(kubeIngress)
	if !reflect.DeepEqual(i, got) {
		t.Errorf("mapping from kubernetes ingress to adapter failed. wanted %v, got %v", i, got)
	}
	if got.CertificateARN() != kubeIngress.Metadata.Annotations[ingressCertificateARNAnnotation] {
		t.Error("wrong value from CertificateARN()")
	}
	if got.Scheme() != kubeIngress.Metadata.Annotations[ingressSchemeAnnotation] {
		t.Error("wrong value from Scheme()")
	}
	if got.Hostname() != kubeIngress.Status.LoadBalancer.Ingress[1].Hostname {
		t.Error("wrong value from Hostname()")
	}
	if got.String() != "default/foo" {
		t.Error("wrong value from String()")
	}

	kubeIngress.Status.LoadBalancer.Ingress = kubeIngress.Status.LoadBalancer.Ingress[1:]
	gotKube := newIngressForKube(got)
	if !reflect.DeepEqual(kubeIngress, gotKube) {
		t.Errorf("mapping from adapter to kubernetes ingress failed. wanted %v, got %v", kubeIngress, gotKube)
	}
}

func TestNodeMapping(t *testing.T) {
	expectedNode := &Node{
		name:       "ip-1.2.3.4.ec2.internal",
		internalIp: "1.2.3.4",
	}

	kubeNode := &node{
		Metadata: nodeMetadata{
			Name:            "ip-1.2.3.4.ec2.internal",
			Uid:             "fixture02",
			Annotations:     map[string]interface{}{},
			SelfLink:        "/api/v1/nodes/ip-1.2.3.4.ec2.internal",
			ResourceVersion: "42",
			Labels:          map[string]interface{}{"kubernetes.io/role": "node"},
		},
		Status: nodeStatus{
			Addresses: []nodeAddress{
				nodeAddress{
					Address: "1.2.3.4",
					Type:    nodeAddressTypeInternalIP,
				},
				nodeAddress{
					Address: "ip-1.2.3.4.ec2.internal",
					Type:    nodeAddressTypeHostname,
				},
			},
		},
		Spec: nodeSpec{
			ExternalId: "i-0123456789abcdef0",
			PodCIDR:    "100.100.100.0/24",
			ProviderId: "aws:///us-east-1c/i-0123456789abcdef0",
		},
	}

	got := newNodeFromKube(kubeNode)
	if !reflect.DeepEqual(expectedNode, got) {
		t.Errorf("mapping from kubernetes node to adapter failed. wanted %v, got %v", expectedNode, got)
	}
	if got.InternalIp() != kubeNode.getAddress(nodeAddressTypeInternalIP) {
		t.Error("wrong value from InternalIp()")
	}
	if got.Name() != kubeNode.Metadata.Name {
		t.Error("wrong value from Name()")
	}
}

func TestCertDomainAnnotation(t *testing.T) {
	certDomain := "foo.org"

	kubeMeta := ingressItemMetadata{
		Namespace: "default",
		Name:      "foo",
		Annotations: map[string]interface{}{
			ingressCertificateARNAnnotation:    "zbr",
			ingressCertificateDomainAnnotation: certDomain,
		},
	}
	kubeIngress := &ingress{
		Metadata: kubeMeta,
	}

	got := newIngressFromKube(kubeIngress)
	if got.CertHostname() != certDomain {
		t.Errorf("expected cert hostname %s, got %s", certDomain, got.CertHostname())
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
	broken   bool
	testFile string
}

func (c *mockClient) get(res string) (io.ReadCloser, error) {
	if !c.broken && (res == ingressListResource || res == nodeListResource) {
		buf, err := ioutil.ReadFile(c.testFile)
		if err != nil {
			return nil, err
		}
		return ioutil.NopCloser(bytes.NewReader(buf)), nil
	}
	return nil, errors.New("mocked error")
}

func (c *mockClient) patch(res string, payload []byte) (io.ReadCloser, error) {
	if !c.broken && res == "/apis/extensions/v1beta1/namespaces/default/ingresses/foo/status" {
		return ioutil.NopCloser(strings.NewReader(":)")), nil
	}
	return nil, errors.New("mocked error")
}

var testConfig = InsecureConfig("dummy-url")

func TestListIngress(t *testing.T) {
	a, _ := NewAdapter(testConfig)
	client := &mockClient{testFile: "testdata/fixture01.json"}
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

func TestListNode(t *testing.T) {
	a, _ := NewAdapter(testConfig)
	client := &mockClient{testFile: "testdata/fixture02.json"}
	a.kubeClient = client
	nodes, err := a.ListNode()
	if err != nil {
		t.Error(err)
	}
	if len(nodes) != 1 {
		t.Fatal("unexpected count of node resources")
	}
	client.broken = true
	_, err = a.ListNode()
	if err == nil {
		t.Error("expected an error")
	}
}

func TestUpdateIngressLoadBalancer(t *testing.T) {
	a, _ := NewAdapter(testConfig)
	client := &mockClient{testFile: "testdata/fixture01.json"}
	a.kubeClient = client
	ing := &Ingress{
		namespace:      "default",
		name:           "foo",
		hostName:       "bar",
		certificateARN: "zbr",
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
			_, err := NewAdapter(test.cfg)
			if err == nil {
				t.Error("expected an error")
			}
		})
	}
}
