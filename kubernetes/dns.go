package kubernetes

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	apiVersion = "externaldns.k8s.io/v1alpha1"
)

type endpoint struct {
	// The hostname of the DNS record
	DNSName string `json:"dnsName,omitempty"`
	// RecordType type of record, e.g. CNAME, A, SRV, TXT etc
	RecordType string `json:"recordType,omitempty"`
	// The targets the DNS record points to
	Targets []string `json:"targets,omitempty"`
}

type dnsEndpointSpec struct {
	Endpoints []*endpoint `json:"endpoints,omitempty"`
}

type dnsEndpoint struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec dnsEndpointSpec `json:"spec,omitempty"`
}

type dnsEndpointPatch struct {
	Spec dnsEndpointSpec `json:"spec,omitempty"`
}

func updateHostnames(c client, loadBalancerHostnames map[string]string) error {
	// TODO: naming
	namespace := "kube-system"
	name := "kube-ingress-aws-controller-dns"

	actualSpec := newEndpointSpec(loadBalancerHostnames)

	existing, err := getEndpoint(c, namespace, name)
	if err != nil {
		if errors.Is(err, ErrResourceNotFound) {
			return createEndpoint(c, namespace, name, actualSpec)
		}
		return err
	}

	if reflect.DeepEqual(existing.Spec, actualSpec) {
		return ErrUpdateNotNeeded
	}

	return patchEndpointSpec(c, namespace, name, actualSpec)
}

func newEndpointSpec(loadBalancerHostnames map[string]string) dnsEndpointSpec {
	spec := dnsEndpointSpec{}
	for hostname, loadBalancerDNSName := range loadBalancerHostnames {
		spec.Endpoints = append(spec.Endpoints, &endpoint{
			DNSName:    hostname,
			RecordType: "CNAME",
			Targets:    []string{loadBalancerDNSName},
		})
	}
	sort.Slice(spec.Endpoints, func(i, j int) bool {
		return spec.Endpoints[i].DNSName < spec.Endpoints[j].DNSName
	})
	return spec
}

func getEndpoint(c client, namespace, name string) (*dnsEndpoint, error) {
	r, err := c.get(endpointResource(namespace, name))
	if err != nil {
		return nil, err
	}
	defer r.Close()

	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	var ep dnsEndpoint
	if err := json.Unmarshal(b, &ep); err != nil {
		return nil, err
	}
	return &ep, nil
}

func createEndpoint(c client, namespace, name string, spec dnsEndpointSpec) error {
	ep := &dnsEndpoint{
		TypeMeta: metav1.TypeMeta{
			APIVersion: apiVersion,
			Kind:       "DNSEndpoint",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: spec,
	}

	payload, err := json.Marshal(ep)
	if err != nil {
		return err
	}

	r, err := c.post(endpointListResource(namespace), payload)
	if err != nil {
		return err
	}
	r.Close()

	return nil
}

func patchEndpointSpec(c client, namespace, name string, spec dnsEndpointSpec) error {
	p := &dnsEndpointPatch{
		Spec: spec,
	}

	payload, err := json.Marshal(p)
	if err != nil {
		return err
	}

	r, err := c.patch(endpointResource(namespace, name), payload)
	if err != nil {
		return err
	}
	r.Close()

	return nil
}

func endpointListResource(namespace string) string {
	return fmt.Sprintf("/apis/%s/namespaces/%s/dnsendpoints", apiVersion, namespace)
}

func endpointResource(namespace, name string) string {
	return fmt.Sprintf("/apis/%s/namespaces/%s/dnsendpoints/%s", apiVersion, namespace, name)
}
