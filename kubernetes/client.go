package kubernetes

import (
	"io"
	"net/http"
	"strings"
)

type Client struct {
	baseURL string
}

const (
	// Usable with kubectl proxy
	defaultHostname = "http://127.0.0.1:8001"
)

var defaultClient = NewClient(defaultHostname)

// Wraps whatever strategy is used to access the Kubernetes API server
// TODO: also allow cert based and bearer token auth
func NewClient(baseURL string) *Client {
	return &Client{baseURL: baseURL}
}

// Can be used for simple Kubernetes API requests that don't have any payload and require a simple GET
func (c *Client) Get(resource string) (io.ReadCloser, error) {
	resp, err := http.Get(c.baseURL + resource)
	if err != nil {
		return nil, err
	}

	return resp.Body, nil
}

// Can be used for more complex requests where a payload needs to follow a PATCH request
func (c *Client) Patch(resource string, payload string) (io.ReadCloser, error) {
	urlStr := c.baseURL + resource
	req, err := http.NewRequest("PATCH", urlStr, strings.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/strategic-merge-patch+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}
