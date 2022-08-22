package kubernetes

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func TestListIngresses(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		f, _ := os.Open("testdata/fixture01.json")
		defer f.Close()
		rw.WriteHeader(http.StatusOK)
		_, err := io.Copy(rw, f)
		require.NoError(t, err)
	}))
	defer testServer.Close()
	kubeClient, _ := newSimpleClient(&Config{BaseURL: testServer.URL}, false)
	ingressClient := &ingressClient{apiVersion: IngressAPIVersionNetworking}
	want := newList(IngressAPIVersionNetworking,
		newIngress("fixture01", nil, "", "example.org", "fixture01", IngressAPIVersionNetworking),
		newIngress("fixture02", map[string]string{ingressClassAnnotation: "skipper"}, "", "skipper.example.org", "fixture02", IngressAPIVersionNetworking),
		newIngress("fixture03", map[string]string{ingressClassAnnotation: "other"}, "", "other.example.org", "fixture03", IngressAPIVersionNetworking),
		newIngress("fixture04", nil, "another", "another.example.org", "fixture04", IngressAPIVersionNetworking),
		newIngress("fixture05", map[string]string{ingressClassAnnotation: "yet-another-ignored"}, "yet-another", "yet-another.example.org", "fixture05", IngressAPIVersionNetworking),
	)
	got, err := ingressClient.listIngress(kubeClient)
	if err != nil {
		t.Errorf("unexpected error from listIngresses: %v", err)
	} else {
		if !reflect.DeepEqual(got, want) {
			t.Errorf("unexpected result from listIngresses. wanted %v, got %v", want, got)
			t.Errorf("%v", cmp.Diff(want, got))
		}
	}
}

func TestListIngressesV1(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		f, _ := os.Open("testdata/fixture03.json")
		defer f.Close()
		rw.WriteHeader(http.StatusOK)
		_, err := io.Copy(rw, f)
		require.NoError(t, err)
	}))
	defer testServer.Close()
	kubeClient, _ := newSimpleClient(&Config{BaseURL: testServer.URL}, false)
	ingressClient := &ingressClient{apiVersion: IngressAPIVersionNetworkingV1}
	want := newList(IngressAPIVersionNetworkingV1,
		newIngress("fixture01", nil, "", "example.org", "fixture01", IngressAPIVersionNetworkingV1),
		newIngress("fixture02", map[string]string{ingressClassAnnotation: "skipper"}, "", "skipper.example.org", "fixture02", IngressAPIVersionNetworkingV1),
		newIngress("fixture03", map[string]string{ingressClassAnnotation: "other"}, "", "other.example.org", "fixture03", IngressAPIVersionNetworkingV1),
		newIngress("fixture04", nil, "another", "another.example.org", "fixture04", IngressAPIVersionNetworkingV1),
	)
	got, err := ingressClient.listIngress(kubeClient)
	if err != nil {
		t.Errorf("unexpected error from listIngresses: %v", err)
	} else {
		if !reflect.DeepEqual(got, want) {
			t.Errorf("unexpected result from listIngresses. wanted %v, got %v", want, got)
			t.Errorf("%v", cmp.Diff(want, got))
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
			kubeClient, _ := newSimpleClient(cfg, false)
			ingressClient := &ingressClient{apiVersion: IngressAPIVersionNetworking}

			_, err := ingressClient.listIngress(kubeClient)
			if err == nil {
				t.Error("expected an error but list ingress call succeeded")
			}
		})
	}
}

func TestUpdateIngressLoadBalancer(t *testing.T) {
	expectedContentType := map[string]bool{
		"application/json-patch+json":            true,
		"application/merge-patch+json":           true,
		"application/strategic-merge-patch+json": true,
	}
	testServer := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.URL.Path != fmt.Sprintf(ingressPatchStatusResource, IngressAPIVersionNetworking, "foo", "bar") {
			t.Error("unexpected URL path sent by the client", req.URL.Path)
		}
		if req.Method != "PATCH" {
			t.Error("unexpected HTTP method. Wanted PATCH but got", req.Method)
		}
		ct := req.Header.Get("Content-Type")
		if !expectedContentType[ct] {
			t.Error("unexpected content type", ct)
		}
		b, err := io.ReadAll(req.Body)
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
	kubeClient, _ := newSimpleClient(cfg, false)
	ingressClient := &ingressClient{apiVersion: IngressAPIVersionNetworking}

	if err := ingressClient.updateIngressLoadBalancer(kubeClient, "foo", "bar", "example.org"); err != nil {
		t.Error("unexpected result from update call:", err)
	}
}

func TestUpdateIngressFailureScenarios(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusInternalServerError)
	}))
	defer testServer.Close()
	cfg := &Config{BaseURL: testServer.URL}
	kubeClient, _ := newSimpleClient(cfg, false)
	ingressClient := &ingressClient{apiVersion: IngressAPIVersionNetworking}
	for _, test := range []struct {
		ing *ingress
	}{
		{newIngress("foo", nil, "", "example.org", "", IngressAPIVersionNetworking)},
	} {
		arn := getAnnotationsString(test.ing.Metadata.Annotations, ingressCertificateARNAnnotation, "<missing>")
		t.Run(fmt.Sprintf("%v/%v", test.ing.Status.LoadBalancer.Ingress[0].Hostname, arn), func(t *testing.T) {
			err := ingressClient.updateIngressLoadBalancer(kubeClient, test.ing.Metadata.Namespace, test.ing.Metadata.Name, "example.com")
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

func newList(version string, ingresses ...*ingress) *ingressList {
	ret := ingressList{
		APIVersion: version,
		Kind:       "IngressList",
		Metadata: ingressListMetadata{
			SelfLink:        fmt.Sprintf("/apis/%s/ingresses", version),
			ResourceVersion: "42",
		},
		Items: ingresses,
	}
	return &ret
}

func newIngress(name string, annotations map[string]string, ingressClassName string, hostname, arn, apiVersion string) *ingress {
	ret := ingress{
		Metadata: kubeItemMetadata{
			Name:              name,
			Namespace:         "default",
			Annotations:       annotations,
			ResourceVersion:   "42",
			SelfLink:          fmt.Sprintf("/apis/%s/namespaces/default/ingresses/", apiVersion) + name,
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
	if ingressClassName != "" {
		ret.Spec.IngressClassName = ingressClassName
	}
	return &ret
}
