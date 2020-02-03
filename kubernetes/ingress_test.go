package kubernetes

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestListIngresses(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		f, _ := os.Open("testdata/fixture01.json")
		defer f.Close()
		rw.WriteHeader(http.StatusOK)
		io.Copy(rw, f)
	}))
	defer testServer.Close()
	kubeClient, _ := newSimpleClient(&Config{BaseURL: testServer.URL})
	want := newList(
		newIngress("fixture01", nil, "example.org", "fixture01"),
		newIngress("fixture02", map[string]string{ingressClassAnnotation: "skipper"}, "skipper.example.org", "fixture02"),
		newIngress("fixture03", map[string]string{ingressClassAnnotation: "other"}, "other.example.org", "fixture03"),
	)
	got, err := listIngress(kubeClient)
	if err != nil {
		t.Errorf("unexpected error from listIngresses: %v", err)
	} else {
		if !reflect.DeepEqual(got, want) {
			t.Errorf("unexpected result from listIngresses. wanted %v, got %v", want, got)
		}
	}
}

func TestListIngressFailureScenarios(t *testing.T) {
	for _, test := range []struct {
		statusCode int
		body       string
	}{
		{http.StatusInternalServerError, "{}"},
		{http.StatusOK, "`"},
	} {
		t.Run(fmt.Sprintf("%v", test.statusCode), func(t *testing.T) {
			testServer := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
				rw.WriteHeader(test.statusCode)
				fmt.Fprintln(rw, test.body)
			}))
			defer testServer.Close()
			cfg := &Config{BaseURL: testServer.URL}
			kubeClient, _ := newSimpleClient(cfg)

			_, err := listIngress(kubeClient)
			if err == nil {
				t.Error("expected an error but list ingress call succeeded")
			}
		})
	}
}

func TestUpdateIngressLoaBalancer(t *testing.T) {
	expectedContentType := map[string]bool{
		"application/json-patch+json":            true,
		"application/merge-patch+json":           true,
		"application/strategic-merge-patch+json": true,
	}
	testServer := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.URL.Path != fmt.Sprintf(ingressPatchStatusResource, "foo", "bar") {
			t.Error("unexpected URL path sent by the client", req.URL.Path)
		}
		if req.Method != "PATCH" {
			t.Error("unexpected HTTP method. Wanted PATCH but got", req.Method)
		}
		ct := req.Header.Get("Content-Type")
		if !expectedContentType[ct] {
			t.Error("unexpected content type", ct)
		}
		b, err := ioutil.ReadAll(req.Body)
		if err != nil {
			t.Error(err)
		}
		got := string(b)
		expected := `{"status":{"loadBalancer":{"ingress":[{"hostname":"example.org"}]}}}`
		if got != expected {
			t.Errorf("unexpected request body. Wanted %s but got %s", expected, got)
		}
		rw.WriteHeader(http.StatusOK)
	}))
	defer testServer.Close()
	cfg := &Config{BaseURL: testServer.URL}
	kubeClient, _ := newSimpleClient(cfg)
	ing := &ingress{
		Metadata: kubeItemMetadata{
			Namespace: "foo",
			Name:      "bar",
		},
	}

	if err := updateIngressLoadBalancer(kubeClient, ing, "example.org"); err != nil {
		t.Error("unexpected result from update call:", err)
	}
}

func TestUpdateIngressFailureScenarios(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusInternalServerError)
	}))
	defer testServer.Close()
	cfg := &Config{BaseURL: testServer.URL}
	kubeClient, _ := newSimpleClient(cfg)
	for _, test := range []struct {
		ing *ingress
	}{
		{newIngress("foo", nil, "example.com", "")},
		{newIngress("foo", nil, "example.org", "")},
	} {
		arn := getAnnotationsString(test.ing.Metadata.Annotations, ingressCertificateARNAnnotation, "<missing>")
		t.Run(fmt.Sprintf("%v/%v", test.ing.Status.LoadBalancer.Ingress[0].Hostname, arn), func(t *testing.T) {
			err := updateIngressLoadBalancer(kubeClient, test.ing, "example.com")
			if err == nil {
				t.Error("expected an error but update ingress call succeeded")
			}
		})
	}
}

func TestAnnotationsFallback(t *testing.T) {
	have := &ingress{Metadata: kubeItemMetadata{Annotations: map[string]string{"foo": "bar"}}}
	for _, test := range []struct {
		key      string
		fallback string
		want     string
	}{
		{"foo", "zbr", "bar"},
		{"missing", "fallback", "fallback"},
	} {
		t.Run(fmt.Sprintf("%s/%s/%s", test.key, test.want, test.fallback), func(t *testing.T) {
			if got := getAnnotationsString(have.Metadata.Annotations, test.key, test.fallback); got != test.want {
				t.Errorf("unexpected metadata value. wanted %q, got %q", test.want, got)
			}
		})
	}
}

func newList(ingresses ...*ingress) *ingressList {
	ret := ingressList{
		APIVersion: "extensions/v1beta1",
		Kind:       "IngressList",
		Metadata: ingressListMetadata{
			SelfLink:        "/apis/extensions/v1beta1/ingresses",
			ResourceVersion: "42",
		},
		Items: ingresses,
	}
	return &ret
}

func newIngress(name string, annotations map[string]string, hostname string, arn string) *ingress {
	ret := ingress{
		Metadata: kubeItemMetadata{
			Name:              name,
			Namespace:         "default",
			Annotations:       annotations,
			ResourceVersion:   "42",
			SelfLink:          "/apis/extensions/v1beta1/namespaces/default/ingresses/" + name,
			Generation:        1,
			UID:               name,
			CreationTimestamp: time.Date(2016, 11, 29, 14, 53, 42, 0, time.UTC),
		},
	}
	if arn != "" {
		if annotations == nil {
			ret.Metadata.Annotations = map[string]string{ingressCertificateARNAnnotation: arn}
		} else {
			ret.Metadata.Annotations[ingressCertificateARNAnnotation] = arn
		}
	}
	if hostname != "" {
		ret.Status.LoadBalancer = ingressLoadBalancerStatus{
			Ingress: []ingressLoadBalancer{{Hostname: hostname}},
		}
	}
	return &ret
}
