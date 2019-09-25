package kubernetes

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
)

func TestGetConfigMap(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		f, _ := os.Open("testdata/fixture02.json")
		defer f.Close()
		rw.WriteHeader(http.StatusOK)
		io.Copy(rw, f)
	}))
	defer testServer.Close()

	kubeClient, _ := newSimpleClient(&Config{BaseURL: testServer.URL})

	want := newConfigMap("foo-ns", "foo-name", map[string]string{
		"some-key": "key1: val1\nkey2: val2\n",
	})

	got, err := getConfigMap(kubeClient, "foo-ns", "foo-name")
	if err != nil {
		t.Errorf("unexpected error from getConfigMap: %v", err)
	} else {
		if !reflect.DeepEqual(got, want) {
			t.Errorf("unexpected result from getConfigMap. wanted %v, got %v", want, got)
		}
	}
}

func TestGetConfigMapFailureScenarios(t *testing.T) {
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

			kubeClient, _ := newSimpleClient(&Config{BaseURL: testServer.URL})

			_, err := getConfigMap(kubeClient, "foo-ns", "foo-name")
			if err == nil {
				t.Error("expected an error but getConfigMap call succeeded")
			}
		})
	}
}

func newConfigMap(namespace, name string, data map[string]string) *configMap {
	return &configMap{
		Kind:       "ConfigMap",
		APIVersion: "v1",
		Metadata: configMapMetadata{
			Name:      name,
			Namespace: namespace,
		},
		Data: data,
	}
}
