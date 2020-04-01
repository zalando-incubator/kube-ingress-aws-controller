package kubernetes

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/linki/instrumented_http"
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

const defaultControllerUserAgent = "kube-ingress-aws-controller"

func newSimpleClient(cfg *Config) (client, error) {
	var (
		tlsConfig *tls.Config
		transport http.RoundTripper = http.DefaultTransport
		c         *http.Client      = http.DefaultClient
	)
	if cfg.CAFile != "" {
		fileData, err := ioutil.ReadFile(cfg.CAFile)
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
			TLSHandshakeTimeout: 10 * time.Second,
			TLSClientConfig:     tlsConfig,
			Dial: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).Dial,
		}
	}

	if transport != http.DefaultTransport {
		c = &http.Client{Transport: transport}
		if cfg.Timeout > 0 {
			c.Timeout = cfg.Timeout
		}
	}

	c = instrumented_http.NewClient(c, &instrumented_http.Callbacks{
		PathProcessor: func(path string) string {
			parts := strings.Split(path, "/")
			return parts[len(parts)-1]
		},
	})

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
	b, err := ioutil.ReadAll(resp.Body)
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
		b, err := ioutil.ReadAll(resp.Body)
		if err == nil {
			err = fmt.Errorf("unexpected status code (%s) for PATCH %q: %s", http.StatusText(resp.StatusCode), resource, b)
		}

		resp.Body.Close()
		return ioutil.NopCloser(bytes.NewBuffer(b)), err
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
	if c.cfg.BearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.BearerToken)
	}
	return req, nil
}
