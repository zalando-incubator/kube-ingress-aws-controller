package kubernetes

import (
	"io/ioutil"
	"net"
	"os"
	"time"
)

// Config holds the common attributes that can be passed to a Kubernetes client on
// initialization.
//
// Mostly copied from https://github.com/kubernetes/client-go/blob/master/rest/config.go
type Config struct {
	// BaseURL must be a URL to the base of the apiserver.
	BaseURL string

	// Server requires Bearer authentication. This client will not attempt to use
	// refresh tokens for an OAuth2 flow.
	// TODO: demonstrate an OAuth2 compatible client.
	BearerToken string

	// TLSClientConfig contains settings to enable transport layer security
	TLSClientConfig

	// Server should be accessed without verifying the TLS
	// certificate. For testing only.
	Insecure bool

	// UserAgent is an optional field that specifies the caller of this request.
	UserAgent string

	// The maximum length of time to wait before giving up on a server request. A value of zero means no timeout.
	Timeout time.Duration
}

// TLSClientConfig contains settings to enable transport layer security
type TLSClientConfig struct {
	// Trusted root certificates for server
	CAFile string
}

const (
	serviceAccountDir       = "/var/run/secrets/kubernetes.io/serviceaccount/"
	serviceAccountTokenKey  = "token"
	serviceAccountRootCAKey = "ca.crt"

	serviceHostEnvVar = "KUBERNETES_SERVICE_HOST"
	servicePortEnvVar = "KUBERNETES_SERVICE_PORT"
)

var serviceAccountLocator = defaultServiceAccountLocator

func defaultServiceAccountLocator() string { return serviceAccountDir }

// InClusterConfig creates a configuration for the Kubernetes Adapter that will communicate with the API server
// using TLS and authenticate with the cluster provide Bearer token.
// The environment should contain variables KUBERNETES_SERVICE_HOST and KUBERNETES_SERVICE_PORT.
// The CA certificate and Bearer token will also be taken from the Kubernetes environment.
func InClusterConfig() (*Config, error) {
	host, port := os.Getenv(serviceHostEnvVar), os.Getenv(servicePortEnvVar)
	if len(host) == 0 || len(port) == 0 {
		return nil, ErrMissingKubernetesEnv
	}

	dir := serviceAccountLocator()
	token, err := ioutil.ReadFile(dir + serviceAccountTokenKey)
	if err != nil {
		return nil, err
	}
	rootCAFile := dir + serviceAccountRootCAKey
	if _, err := os.Stat(rootCAFile); os.IsNotExist(err) {
		return nil, err
	}

	return &Config{
		BaseURL:         "https://" + net.JoinHostPort(host, port),
		UserAgent:       "kube-ingress-aws-controller",
		BearerToken:     string(token),
		Timeout:         10 * time.Second,
		TLSClientConfig: TLSClientConfig{CAFile: rootCAFile},
	}, nil
}

// InsecureConfig creates a configuration for the Kubernetes Adapter that won't use any encryption or authentication
// mechanisms to communicate with the API Server. This should be used only for local development, as usually provided
// by  the kubectl proxy
func InsecureConfig(apiServerBaseURL string) *Config {
	return &Config{
		BaseURL:   apiServerBaseURL,
		UserAgent: defaultControllerUserAgent,
		Timeout:   10 * time.Second,
	}
}
