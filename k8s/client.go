package k8s

import (
	"net/http"
	"io"
	"fmt"
)

type kubeClient struct {
	hostname string
}

const (
	defaultHostname = "http://127.0.0.1:8001"
)

func defaultKubeClient() (*kubeClient, error) {
	return newKubeClient(defaultHostname)
}

func newKubeClient(hostname string) (*kubeClient, error) {
	return &kubeClient{hostname: hostname}, nil
}

func (c *kubeClient) Get(resource string) (io.ReadCloser, error) {
	resp, err := http.Get(fmt.Sprintf("%s/%s", c.hostname, resource))
	if err != nil {
		return nil, err
	}

	return resp.Body, nil
}