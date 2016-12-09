package kubernetes

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
)

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
	kubeClient := NewClient(testServer.URL)
	ingresses := []Ingress{
		{
			Metadata: listMeta{
				Namespace: "foo",
				Name:      "bar",
			},
		},
	}

	if err := UpdateIngressLoaBalancer(kubeClient, ingresses, "example.org"); err != nil {
		t.Error("unexpected result from update call:", err)
	}
}
