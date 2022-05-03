package kubernetes

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zalando-incubator/kube-ingress-aws-controller/kubernetes/apitest"
)

func TestUpdateHostnames(t *testing.T) {
	for _, test := range []struct {
		name          string
		hostnames     map[string]string
		handlers      map[string]apitest.ApiHandler
		expectedError string
	}{
		{
			name: "successful create",
			hostnames: map[string]string{
				"foo.test": "kube-ing-lb-1",
				"bar.test": "kube-ing-lb-2",
			},
			handlers: map[string]apitest.ApiHandler{
				"GET /kube-system/dnsendpoints/kube-ingress-aws-controller-dns": func(t *testing.T, w http.ResponseWriter, r *http.Request) {
					http.Error(w, "Not Found", http.StatusNotFound)
				},
				"POST /kube-system/dnsendpoints": func(t *testing.T, w http.ResponseWriter, r *http.Request) {
					apitest.ExpectJsonBodyAsYaml(t, r, "testdata/dns/foo-bar.yaml")
					w.WriteHeader(http.StatusCreated)
				},
			},
		},
		{
			name: "successful update",
			hostnames: map[string]string{
				"foo.test": "kube-ing-lb-3",
				"bar.test": "kube-ing-lb-3",
				"baz.test": "kube-ing-lb-3",
			},
			handlers: map[string]apitest.ApiHandler{
				"GET /kube-system/dnsendpoints/kube-ingress-aws-controller-dns": apitest.JsonFromYamlHandler("testdata/dns/foo-bar.yaml"),
				"PATCH /kube-system/dnsendpoints/kube-ingress-aws-controller-dns": func(t *testing.T, w http.ResponseWriter, r *http.Request) {
					apitest.ExpectJsonBodyAsYaml(t, r, "testdata/dns/foo-bar-baz-patch.yaml")
				},
			},
		},
		{
			name: "update not needed",
			hostnames: map[string]string{
				"foo.test": "kube-ing-lb-1",
				"bar.test": "kube-ing-lb-2",
			},
			handlers: map[string]apitest.ApiHandler{
				"GET /kube-system/dnsendpoints/kube-ingress-aws-controller-dns": apitest.JsonFromYamlHandler("testdata/dns/foo-bar.yaml"),
			},
			expectedError: ErrUpdateNotNeeded.Error(),
		},
		{
			name: "get endpoint error",
			hostnames: map[string]string{
				"foo.test": "kube-ing-lb-1",
				"bar.test": "kube-ing-lb-2",
			},
			handlers: map[string]apitest.ApiHandler{
				"GET /kube-system/dnsendpoints/kube-ingress-aws-controller-dns": func(t *testing.T, w http.ResponseWriter, r *http.Request) {
					http.Error(w, "oops", http.StatusInternalServerError)
				},
			},
			expectedError: "unexpected status code (Internal Server Error) for GET \"/apis/externaldns.k8s.io/v1alpha1/namespaces/kube-system/dnsendpoints/kube-ingress-aws-controller-dns\": oops\n",
		},
		{
			name: "patch endpoint error",
			hostnames: map[string]string{
				"foo.test": "kube-ing-lb-3",
				"bar.test": "kube-ing-lb-3",
				"baz.test": "kube-ing-lb-3",
			},
			handlers: map[string]apitest.ApiHandler{
				"GET /kube-system/dnsendpoints/kube-ingress-aws-controller-dns": apitest.JsonFromYamlHandler("testdata/dns/foo-bar.yaml"),
				"PATCH /kube-system/dnsendpoints/kube-ingress-aws-controller-dns": func(t *testing.T, w http.ResponseWriter, r *http.Request) {
					http.Error(w, "oops", http.StatusInternalServerError)
				},
			},
			expectedError: "unexpected status code (Internal Server Error) for PATCH \"/apis/externaldns.k8s.io/v1alpha1/namespaces/kube-system/dnsendpoints/kube-ingress-aws-controller-dns\": oops\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := apitest.NewApiServer(t, "/apis/externaldns.k8s.io/v1alpha1/namespaces", test.handlers)
			defer s.Close()

			cfg := &Config{BaseURL: s.URL}
			kubeClient, _ := newSimpleClient(cfg, false)

			err := updateHostnames(kubeClient, test.hostnames)
			if test.expectedError == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, test.expectedError)
			}
		})
	}
}
