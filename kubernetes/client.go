package kubernetes

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
)

// Client wraps the strategies used to access a Kubernetes API server.
// TODO: also allow cert based and bearer token auth
type Client struct {
	baseURL string
}

// NewClient returns a new simple client to a Kubernetes API server using only its base URL. It should be enough to
// access an API endpoint via a proxy which takes care of all the authentication details, like `kubectl proxy`.
func NewClient(baseURL string) *Client {
	return &Client{baseURL: baseURL}
}

// Get can be used for simple Kubernetes API requests that don't have any payload and require a simple GET.
func (c *Client) Get(resource string) (io.ReadCloser, error) {
	resp, err := http.Get(c.baseURL + resource)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		var err error
		b, err := ioutil.ReadAll(resp.Body)
		if err == nil {
			err = fmt.Errorf("unexpected status code (%s) for GET %q: %s", http.StatusText(resp.StatusCode), resource, b)
		}
		resp.Body.Close()
		return nil, err
	}
	return resp.Body, nil
}

// Patch can be used for more complex requests where a payload needs to follow a PATCH request.
func (c *Client) Patch(resource string, payload []byte) (io.ReadCloser, error) {
	urlStr := c.baseURL + resource
	req, err := http.NewRequest("PATCH", urlStr, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/strategic-merge-patch+json")
	resp, err := http.DefaultClient.Do(req)
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
		return nil, err
	}
	return resp.Body, nil
}
