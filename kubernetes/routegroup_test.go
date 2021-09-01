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

func TestListRoutegroups(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		f, _ := os.Open("testdata/fixture01_rg.json")
		defer f.Close()
		rw.WriteHeader(http.StatusOK)
		io.Copy(rw, f)
	}))
	defer testServer.Close()
	kubeClient, _ := newSimpleClient(&Config{BaseURL: testServer.URL}, false)
	want := newRouteGroupList(
		newRoutegroup("fixture-rg01", nil, "example.org", "fixture-rg01"),
		newRoutegroup("fixture-rg02", map[string]string{ingressClassAnnotation: "skipper"}, "skipper.example.org", "fixture-rg02"),
		newRoutegroup("fixture-rg03", map[string]string{ingressClassAnnotation: "other"}, "other.example.org", "fixture-rg03"),
	)
	got, err := listRoutegroups(kubeClient)
	if err != nil {
		t.Errorf("unexpected error from listRoutegroups: %v", err)
	} else {
		if !reflect.DeepEqual(got, want) {
			t.Errorf("unexpected result from listRoutegroup. wanted %v, got %v", want, got)
		}
	}
}

func TestListRoutegroupFailureScenarios(t *testing.T) {
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

			_, err := listRoutegroups(kubeClient)
			if err == nil {
				t.Error("expected an error but list routegroup call succeeded")
			}
		})
	}
}

func TestUpdateRoutegroupLoadBalancer(t *testing.T) {
	expectedContentType := map[string]bool{
		"application/json-patch+json":            true,
		"application/merge-patch+json":           true,
		"application/strategic-merge-patch+json": true,
	}
	testServer := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.URL.Path != fmt.Sprintf(routegroupPatchStatusResource, "foo", "bar") {
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
		expected := `{"status":{"loadBalancer":{"routegroup":[{"hostname":"example.org"}]}}}`
		if got != expected {
			t.Errorf("unexpected request body. Wanted %s but got %s", expected, got)
		}
		rw.WriteHeader(http.StatusOK)
	}))
	defer testServer.Close()
	cfg := &Config{BaseURL: testServer.URL}
	kubeClient, _ := newSimpleClient(cfg, false)

	if err := updateRoutegroupLoadBalancer(kubeClient, "foo", "bar", "example.org"); err != nil {
		t.Error("unexpected result from update call:", err)
	}
}

func TestUpdateRoutegroupFailureScenarios(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusInternalServerError)
	}))
	defer testServer.Close()
	cfg := &Config{BaseURL: testServer.URL}
	kubeClient, _ := newSimpleClient(cfg, false)
	for _, test := range []struct {
		rg *routegroup
	}{
		{newRoutegroup("foo", nil, "example.org", "")},
	} {
		arn := getAnnotationsString(test.rg.Metadata.Annotations, ingressCertificateARNAnnotation, "<missing>")
		t.Run(fmt.Sprintf("%v/%v", test.rg.Status.LoadBalancer.Routegroup[0].Hostname, arn), func(t *testing.T) {
			err := updateRoutegroupLoadBalancer(kubeClient, test.rg.Metadata.Namespace, test.rg.Metadata.Name, "example.com")
			if err == nil {
				t.Error("expected an error but update routegroup call succeeded")
			}
		})
	}
}

func TestRouteGroupAnnotationsFallback(t *testing.T) {
	have := &routegroup{Metadata: kubeItemMetadata{Annotations: map[string]string{"foo": "bar"}}}
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

func newRouteGroupList(rgs ...*routegroup) *routegroupList {
	ret := routegroupList{
		APIVersion: "zalando.org/v1",
		Kind:       "RoutegroupList",
		Metadata: routegroupListMetadata{
			SelfLink:        routegroupListResource,
			ResourceVersion: "42",
		},
		Items: rgs,
	}
	return &ret
}

func newRoutegroup(name string, annotations map[string]string, hostname string, arn string) *routegroup {
	ret := routegroup{
		Metadata: kubeItemMetadata{
			Name:              name,
			Namespace:         "default",
			Annotations:       annotations,
			ResourceVersion:   "42",
			SelfLink:          fmt.Sprintf(routegroupNamespacedResource, "default", name),
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
		ret.Status.LoadBalancer = routegroupLoadBalancerStatus{
			Routegroup: []routegroupLoadBalancer{{Hostname: hostname}},
		}
	}
	return &ret
}
