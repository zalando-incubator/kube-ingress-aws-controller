package mock

import (
	"github.com/zalando-incubator/kube-ingress-aws-controller/kubernetes"

	"github.com/stretchr/testify/mock"
)

// API is a mock implementation of [kubernetes.API]
type API struct {
	mock.Mock
}

var _ kubernetes.API = (*API)(nil)

func (m *API) ListResources() ([]*kubernetes.Ingress, error) {
	args := m.Called()
	return args.Get(0).([]*kubernetes.Ingress), args.Error(1)
}

func (m *API) UpdateIngressLoadBalancer(ingress *kubernetes.Ingress, loadBalancerDNSName string) error {
	args := m.Called(ingress, loadBalancerDNSName)
	return args.Error(0)
}

func (m *API) GetConfigMap(namespace, name string) (*kubernetes.ConfigMap, error) {
	args := m.Called(namespace, name)
	return args.Get(0).(*kubernetes.ConfigMap), args.Error(1)
}
