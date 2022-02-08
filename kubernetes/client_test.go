package kubernetes

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

func TestClientGet(t *testing.T) {
	for _, test := range []struct {
		baseURL      string
		resource     string
		responseBody string
		responseCode int
		wantError    bool
	}{
		{"", "/foo", "foo", http.StatusOK, false},
		{"", "/bar", "bar", http.StatusNotFound, true},
		{"", "/zbr", "xpto", http.StatusInternalServerError, true},
		{"http://192.168.0.%31", "/fail", "xpto", http.StatusOK, true},
	} {
		t.Run(fmt.Sprintf("%d%v", test.responseCode, test.resource), func(t *testing.T) {
			handler := func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" {
					t.Errorf("unexpected HTTP method. wanted GET, got %q", r.Method)
					w.WriteHeader(http.StatusBadRequest)
				}
				if r.URL.Path != test.resource {
					t.Errorf("unexpected URL path. wanted %q, got %q", test.resource, r.URL.Path)
					w.WriteHeader(http.StatusBadRequest)
				}
				w.WriteHeader(test.responseCode)
				io.WriteString(w, test.responseBody)
			}

			server := httptest.NewServer(http.HandlerFunc(handler))
			defer server.Close()

			var baseURL = test.baseURL
			if baseURL == "" {
				baseURL = server.URL
			}
			cfg := &Config{BaseURL: baseURL}
			c, _ := newSimpleClient(cfg, false)
			r, err := c.get(test.resource)
			if err != nil && !test.wantError {
				t.Error("got unexpected error", err)
			}

			if !test.wantError {
				b, err := io.ReadAll(r)
				if err != nil {
					t.Error("error reading response", err)
				}
				got := string(b)
				if test.responseBody != got {
					t.Errorf("unexpected response body. wanted %q, got %q\n", test.responseBody, got)
				}
			}
		})
	}
}

func TestClientPatch(t *testing.T) {
	for _, test := range []struct {
		baseURL      string
		resource     string
		payload      []byte
		responseBody string
		responseCode int
		wantError    bool
	}{
		{"", "/foo", []byte("foo"), "ok", http.StatusOK, false},
		{"", "/bar", []byte("bar"), "ok", http.StatusNotFound, true},
		{"", "/zbr", []byte("xpto"), "nok", http.StatusInternalServerError, true},
		{"http://192.168.0.%31", "/fail", []byte("fail"), "nok", http.StatusOK, true},
	} {
		t.Run(fmt.Sprintf("%d%v", test.responseCode, test.resource), func(t *testing.T) {
			handler := func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "PATCH" {
					t.Errorf("unexpected HTTP method. wanted PATCH, got %q", r.Method)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				if r.URL.Path != test.resource {
					t.Errorf("unexpected URL path. wanted %q, got %q", test.resource, r.URL.Path)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				b, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error("failure reading request payload", err)
					return
				}
				if !reflect.DeepEqual(b, test.payload) {
					t.Errorf("unexpected request payload. wanted %v, got %v\n", test.payload, b)
				}
				w.WriteHeader(test.responseCode)
				io.WriteString(w, test.responseBody)
			}

			server := httptest.NewServer(http.HandlerFunc(handler))
			defer server.Close()

			var baseURL = test.baseURL
			if baseURL == "" {
				baseURL = server.URL
			}
			cfg := &Config{BaseURL: baseURL}
			c, _ := newSimpleClient(cfg, false)
			r, err := c.patch(test.resource, test.payload)
			if err != nil && !test.wantError {
				t.Error("got unexpected error", err)
			}

			if !test.wantError {
				b, err := io.ReadAll(r)
				if err != nil {
					t.Error("error reading response", err)
				}
				got := string(b)
				if test.responseBody != got {
					t.Errorf("unexpected response body. wanted %q, got %q\n", test.responseBody, got)
				}
			}
		})
	}
}

func TestTLS(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if token != "Bearer foo" {
			t.Errorf(`wrong auth bearer token. wanted "Bearer: foo" but got %q`, token)
		}
		buf, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		if string(buf) != "bar" {
			t.Errorf(`unexpected request body. wanted "bar" but got %q`, string(buf))
		}
		w.WriteHeader(http.StatusOK)
		io.Copy(w, r.Body)
	}
	cert, err := tls.LoadX509KeyPair("testdata/cert.pem", "testdata/key.pem")
	if err != nil {
		t.Error(err)
	}
	certs := []tls.Certificate{cert}
	server := httptest.NewUnstartedServer(http.HandlerFunc(handler))
	server.TLS = &tls.Config{Certificates: certs}
	server.StartTLS()
	defer server.Close()

	cfg := &Config{
		BaseURL:         server.URL,
		UserAgent:       "kube-ingress-aws-controller",
		BearerToken:     "foo",
		TLSClientConfig: TLSClientConfig{CAFile: "testdata/cert.pem"},
		Timeout:         5 * time.Second,
	}

	c, err := newSimpleClient(cfg, false)
	if err != nil {
		t.Error(err)
	}

	r, err := c.patch("/foo", []byte("bar"))
	if err != nil {
		t.Error(err)
	} else {
		defer r.Close()
	}
}
