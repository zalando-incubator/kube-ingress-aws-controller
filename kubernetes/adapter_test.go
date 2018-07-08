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

func TestMappingRoundtrip(tt *testing.T) {
	for _, tc := range []struct {
		msg         string
		ingress     *Ingress
		kubeIngress *ingress
	}{
		{
			msg: "test parsing a simple ingress object",
			ingress: &Ingress{
				Namespace:      "default",
				Name:           "foo",
				Hostname:       "bar",
				Scheme:         "internal",
				CertificateARN: "zbr",
				Shared:         true,
				Hostnames:      []string{"domain.example.org"},
			},
			kubeIngress: &ingress{
				Metadata: ingressItemMetadata{
					Namespace: "default",
					Name:      "foo",
					Annotations: map[string]interface{}{
						ingressCertificateARNAnnotation: "zbr",
						ingressSchemeAnnotation:         "internal",
						ingressSharedAnnotation:         "true",
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
			msg: "test parsing an ingress object with shared=false annotation",
			ingress: &Ingress{
				Namespace:      "default",
				Name:           "foo",
				Hostname:       "bar",
				Scheme:         "internal",
				CertificateARN: "zbr",
				Shared:         false,
			},
			kubeIngress: &ingress{
				Metadata: ingressItemMetadata{
					Namespace: "default",
					Name:      "foo",
					Annotations: map[string]interface{}{
						ingressCertificateARNAnnotation: "zbr",
						ingressSchemeAnnotation:         "internal",
						ingressSharedAnnotation:         "false",
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
			got := newIngressFromKube(tc.kubeIngress)
			if !reflect.DeepEqual(tc.ingress, got) {
				t.Errorf("mapping from kubernetes ingress to adapter failed. wanted %v, got %v", tc.ingress, got)
			}
			if got.CertificateARN != tc.kubeIngress.Metadata.Annotations[ingressCertificateARNAnnotation] {
				t.Error("wrong value from CertificateARN()")
			}
			if got.Scheme != tc.kubeIngress.Metadata.Annotations[ingressSchemeAnnotation] {
				t.Error("wrong value from Scheme()")
			}
			if got.Hostname != tc.kubeIngress.Status.LoadBalancer.Ingress[1].Hostname {
				t.Error("wrong value from Hostname()")
			}
			if got.String() != fmt.Sprintf("%s/%s", tc.ingress.Namespace, tc.ingress.Name) {
				t.Error("wrong value from String()")
			}

			tc.kubeIngress.Status.LoadBalancer.Ingress = tc.kubeIngress.Status.LoadBalancer.Ingress[1:]
			gotKube := newIngressForKube(got)
			if !reflect.DeepEqual(tc.kubeIngress.Metadata, gotKube.Metadata) {
				t.Errorf("mapping from adapter to kubernetes ingress failed. wanted %v, got %#v", tc.kubeIngress.Metadata, gotKube.Metadata)
			}
			if !reflect.DeepEqual(tc.kubeIngress.Status, gotKube.Status) {
				t.Errorf("mapping from adapter to kubernetes ingress failed. wanted %v, got %#v", tc.kubeIngress.Status, gotKube.Status)
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
	if !c.broken && res == ingressListResource {
		buf, err := ioutil.ReadFile("testdata/fixture01.json")
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
var testIngressFilter = []string{"skipper"}

func TestListIngress(t *testing.T) {
	a, _ := NewAdapter(testConfig, testIngressFilter)
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
	a, _ := NewAdapter(testConfig, testIngressFilter)
	client := &mockClient{}
	a.kubeClient = client
	ing := &Ingress{
		Namespace:      "default",
		Name:           "foo",
		Hostname:       "bar",
		CertificateARN: "zbr",
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
			_, err := NewAdapter(test.cfg, testIngressFilter)
			if err == nil {
				t.Error("expected an error")
			}
		})
	}
}
