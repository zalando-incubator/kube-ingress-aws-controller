package kubernetes

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestListFabricgateways(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		f, _ := os.Open("testdata/fixture01_fg.json")
		defer f.Close()
		rw.WriteHeader(http.StatusOK)
		io.Copy(rw, f)
	}))
	defer testServer.Close()
	kubeClient, _ := newSimpleClient(&Config{BaseURL: testServer.URL}, false)
	want := newFabricgatewayList(
		newFabricgateway("fixture-fg01", nil, "example.org", "fixture-fg01"),
		newFabricgateway("fixture-fg02", map[string]string{ingressClassAnnotation: "skipper"}, "skipper.example.org", "fixture-fg02"),
		newFabricgateway("fixture-fg03", map[string]string{ingressClassAnnotation: "other"}, "other.example.org", "fixture-fg03"),
	)
	got, err := listFabricgateways(kubeClient)
	if err != nil {
		t.Errorf("unexpected error from listFabricgateways: %v", err)
	} else {
		if !cmp.Equal(got, want) {
			t.Errorf("unexpected result from listFabricgateways. %v", cmp.Diff(want, got))
		}
	}
}

func TestListFabricgatewayFailureScenarios(t *testing.T) {
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

			_, err := listFabricgateways(kubeClient)
			if err == nil {
				t.Error("expected an error but list fabric call succeeded")
			}
		})
	}
}

func TestUpdateFabricgatewayLoadBalancer(t *testing.T) {
	expectedContentType := map[string]bool{
		"application/json-patch+json":            true,
		"application/merge-patch+json":           true,
		"application/strategic-merge-patch+json": true,
	}
	testServer := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.URL.Path != fmt.Sprintf(fabricPatchStatusResource, "foo", "bar") {
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
		expected := `{"status":{"loadBalancer":{"fabricgateway":[{"hostname":"example.org"}]}}}`
		if got != expected {
			t.Errorf("unexpected request body. Wanted %s but got %s", expected, got)
		}
		rw.WriteHeader(http.StatusOK)
	}))
	defer testServer.Close()
	cfg := &Config{BaseURL: testServer.URL}
	kubeClient, _ := newSimpleClient(cfg, false)

	if err := updateFabricgatewayLoadBalancer(kubeClient, "foo", "bar", "example.org"); err != nil {
		t.Error("unexpected result from update call:", err)
	}
}

func TestUpdateFabricgatewayFailureScenarios(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusInternalServerError)
	}))
	defer testServer.Close()
	cfg := &Config{BaseURL: testServer.URL}
	kubeClient, _ := newSimpleClient(cfg, false)
	for _, test := range []struct {
		fg *fabric
	}{
		{newFabricgateway("foo", nil, "example.org", "")},
	} {
		arn := getAnnotationsString(test.fg.Metadata.Annotations, ingressCertificateARNAnnotation, "<missing>")
		t.Run(fmt.Sprintf("%v/%v", test.fg.Status.LoadBalancer.Fabric[0].Hostname, arn), func(t *testing.T) {
			err := updateRoutegroupLoadBalancer(kubeClient, test.fg.Metadata.Namespace, test.fg.Metadata.Name, "example.com")
			if err == nil {
				t.Error("expected an error but update routegroup call succeeded")
			}
		})
	}
}

func TestFabricgatewayAnnotationsFallback(t *testing.T) {
	have := &fabric{Metadata: kubeItemMetadata{Annotations: map[string]string{"foo": "bar"}}}
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

func newFabricgatewayList(fgs ...*fabric) *fabricList {
	ret := fabricList{
		APIVersion: "zalando.org/v1",
		Kind:       "FabricgatewayList",
		Metadata: fabricListMetadata{
			SelfLink:        fabricListResource,
			ResourceVersion: "42",
		},
		Items: fgs,
	}
	return &ret
}

func newFabricgateway(name string, annotations map[string]string, hostname string, arn string) *fabric {
	ret := fabric{
		Metadata: kubeItemMetadata{
			Name:              name,
			Namespace:         "default",
			Annotations:       annotations,
			ResourceVersion:   "42",
			SelfLink:          fmt.Sprintf(fabricNamespacedResource, "default", name),
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
		ret.Status.LoadBalancer = fabricLoadBalancerStatus{
			Fabric: []fabricLoadBalancer{{Hostname: hostname}},
		}
	}
	return &ret
}
