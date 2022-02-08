package kubernetes

import (
	"encoding/json"
	"fmt"
	"io"
)

type routegroupList struct {
	Kind       string                 `json:"kind"`
	APIVersion string                 `json:"apiVersion"`
	Metadata   routegroupListMetadata `json:"metadata"`
	Items      []*routegroup          `json:"items"`
}

type routegroup struct {
	Metadata kubeItemMetadata `json:"metadata"`
	Spec     routegroupSpec   `json:"spec"`
	Status   routegroupStatus `json:"status"`
}

type routegroupListMetadata struct {
	SelfLink        string `json:"selfLink"`
	ResourceVersion string `json:"resourceVersion"`
}

type routegroupSpec struct {
	Hosts []string `json:"hosts"`
}

type routegroupStatus struct {
	LoadBalancer routegroupLoadBalancerStatus `json:"loadBalancer"`
}

type routegroupLoadBalancerStatus struct {
	Routegroup []routegroupLoadBalancer `json:"routegroup"`
}

type routegroupLoadBalancer struct {
	Hostname string `json:"hostname"`
}

const (
	routegroupListResource        = "/apis/zalando.org/v1/routegroups"
	routegroupNamespacedResource  = "/apis/zalando.org/v1/namespaces/%s/routegroups/%s"
	routegroupPatchStatusResource = "/apis/zalando.org/v1/namespaces/%s/routegroups/%s/status"
)

func listRoutegroups(c client) (*routegroupList, error) {
	r, err := c.get(routegroupListResource)
	if err != nil {
		return nil, err
	}

	defer r.Close()

	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	var result routegroupList
	if err := json.Unmarshal(b, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

type patchRoutegroupStatus struct {
	Status routegroupStatus `json:"status"`
}

func updateRoutegroupLoadBalancer(c client, ns, name, newHostName string) error {
	patchStatus := patchRoutegroupStatus{
		Status: routegroupStatus{
			LoadBalancer: routegroupLoadBalancerStatus{
				Routegroup: []routegroupLoadBalancer{{Hostname: newHostName}},
			},
		},
	}

	resource := fmt.Sprintf(routegroupPatchStatusResource, ns, name)
	payload, err := json.Marshal(patchStatus)
	if err != nil {
		return err
	}

	r, err := c.patch(resource, payload)
	if err != nil {
		return fmt.Errorf("failed to patch routegroup %s/%s = %q: %w", ns, name, newHostName, err)
	}
	defer r.Close()
	return nil
}
