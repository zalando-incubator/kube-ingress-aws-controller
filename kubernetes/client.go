package kubernetes

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/linki/instrumented_http"
	snet "github.com/zalando/skipper/net"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var ErrResourceNotFound = errors.New("resource not found")
var ErrNoPermissionToAccessResource = errors.New("no permission to access resource")

type client interface {
	get(string) (io.ReadCloser, error)
	patch(string, []byte) (io.ReadCloser, error)
}

type simpleClient struct {
	cfg        *Config
	httpClient *http.Client
}

const (
	defaultControllerUserAgent = "kube-ingress-aws-controller"

	// aligned with https://pkg.go.dev/net/http#DefaultTransport
	maxIdleConns        = 10
	idleConnTimeout     = 90 * time.Second
	timeout             = 30 * time.Second
	tlsHandshakeTimeout = 10 * time.Second
)

func newSimpleClient(cfg *Config, disableInstrumentedHttpClient bool) (client, error) {
	var (
		tlsConfig *tls.Config
		transport http.RoundTripper = http.DefaultTransport
		c         *http.Client      = http.DefaultClient
	)
	if cfg.CAFile != "" {
		fileData, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, err
		}
		certPool := x509.NewCertPool()
		if !certPool.AppendCertsFromPEM(fileData) {
			return nil, ErrInvalidCertificates
		}

		tlsConfig = &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: cfg.Insecure,
			RootCAs:            certPool,
		}
		transport = &http.Transport{
			TLSHandshakeTimeout: tlsHandshakeTimeout,
			TLSClientConfig:     tlsConfig,
			Dial: (&net.Dialer{
				Timeout:   timeout,
				KeepAlive: timeout,
			}).Dial,
		}
	}

	if transport != http.DefaultTransport {
		c = &http.Client{Transport: transport}
		if cfg.Timeout > 0 {
			c.Timeout = cfg.Timeout
		}
	}

	if !disableInstrumentedHttpClient {
		c = instrumented_http.NewClient(c, &instrumented_http.Callbacks{
			PathProcessor: func(path string) string {
				parts := strings.Split(path, "/")
				return parts[len(parts)-1]
			},
		})
	}

	return &simpleClient{cfg: cfg, httpClient: c}, nil
}

func (c *simpleClient) get(resource string) (io.ReadCloser, error) {
	req, err := c.createRequest("GET", resource, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusOK {
		return resp.Body, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrResourceNotFound
	}
	if resp.StatusCode == http.StatusForbidden {
		return nil, ErrNoPermissionToAccessResource
	}
	b, err := io.ReadAll(resp.Body)
	if err == nil {
		err = fmt.Errorf("unexpected status code (%s) for GET %q: %s", http.StatusText(resp.StatusCode), resource, b)
	}
	return nil, err
}

func (c *simpleClient) patch(resource string, payload []byte) (io.ReadCloser, error) {
	req, err := c.createRequest("PATCH", resource, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/merge-patch+json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		var err error
		b, err := io.ReadAll(resp.Body)
		if err == nil {
			err = fmt.Errorf("unexpected status code (%s) for PATCH %q: %s", http.StatusText(resp.StatusCode), resource, b)
		}

		resp.Body.Close()
		return io.NopCloser(bytes.NewBuffer(b)), err
	}
	return resp.Body, nil
}

func (c *simpleClient) createRequest(method, resource string, body io.Reader) (*http.Request, error) {
	urlStr := c.cfg.BaseURL + resource
	req, err := http.NewRequest(method, urlStr, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", defaultControllerUserAgent)
	if c.cfg.UserAgent != "" {
		req.Header.Set("User-Agent", c.cfg.UserAgent)
	}
	if c.cfg.TokenProvider != nil {
		token, ok := c.cfg.TokenProvider.GetSecret(serviceAccountDir + serviceAccountTokenKey)
		if !ok {
			return nil, fmt.Errorf("secret not found: %v", serviceAccountTokenKey)
		}
		req.Header.Set("Authorization", "Bearer "+string(token))
	}
	return req, nil
}

func (a *Adapter) NewInclusterConfigClientset(ctx context.Context) error {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return fmt.Errorf("can't get in cluster config: %w", err)
	}
	// cfg.Timeout = timeout
	trCfg, err := cfg.TransportConfig()
	if err != nil {
		return fmt.Errorf("can't get transport config: %w", err)
	}
	tlsCfg, err := rest.TLSConfigFor(cfg)
	if err != nil {
		return fmt.Errorf("can't get TLS config: %w", err)
	}

	// Todo: can the default Transport be used instead?
	tr := snet.NewTransport(snet.Options{
		IdleConnTimeout: idleConnTimeout,
		Transport: &http.Transport{
			Proxy:                 trCfg.Proxy,
			DialContext:           cfg.Dial,
			TLSClientConfig:       tlsCfg,
			TLSHandshakeTimeout:   tlsHandshakeTimeout,
			DisableKeepAlives:     false,
			DisableCompression:    false,
			MaxIdleConns:          maxIdleConns,
			MaxIdleConnsPerHost:   maxIdleConns,
			IdleConnTimeout:       idleConnTimeout,
			ResponseHeaderTimeout: timeout,
			ExpectContinueTimeout: timeout,
			ForceAttemptHTTP2:     true,
		},
	})
	go func() {
		<-ctx.Done()
		tr.Close()
	}()
	cfg.Transport = tr
	// github.com/kubernetes/client-go/issues/452
	cfg.TLSClientConfig = rest.TLSClientConfig{}
	a.clientset, err = kubernetes.NewForConfig(cfg)
	return err
}
