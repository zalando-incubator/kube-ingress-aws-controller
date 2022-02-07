package kubernetes

import (
	"encoding/json"
	"fmt"
	"io"
)

const (
	fabricListResource        = "/apis/zalando.org/v1/fabricgateways"
	fabricNamespacedResource  = "/apis/zalando.org/v1/namespaces/%s/fabricgateways/%s"
	fabricPatchStatusResource = "/apis/zalando.org/v1/namespaces/%s/fabricgateways/%s/status"
)

type fabricList struct {
	Kind       string             `json:"kind"`
	APIVersion string             `json:"apiVersion"`
	Metadata   fabricListMetadata `json:"metadata"`
	Items      []*fabric          `json:"items"`
}

type fabric struct {
	Metadata kubeItemMetadata `json:"metadata"`
	Spec     fabricSpec       `json:"spec"`
	Status   fabricStatus     `json:"status"`
}

type fabricListMetadata struct {
	SelfLink        string `json:"selfLink"`
	ResourceVersion string `json:"resourceVersion"`
}

type fabricSpec struct {
	ExternalServiceProvider *fabricExternalServiceProvider `json:"x-external-service-provider"`
}

type fabricExternalServiceProvider struct {
	Hosts []string `json:"hosts"`
}

type fabricStatus struct {
	LoadBalancer fabricLoadBalancerStatus `json:"loadBalancer"`
}

type fabricLoadBalancerStatus struct {
	Fabric []fabricLoadBalancer `json:"fabric"`
}

type fabricLoadBalancer struct {
	Hostname string `json:"hostname"`
}

func listFabrics(c client) (*fabricList, error) {
	r, err := c.get(fabricListResource)
	if err != nil {
		return nil, err
	}

	defer r.Close()

	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	var result fabricList
	if err := json.Unmarshal(b, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

type patchFabricStatus struct {
	Status fabricStatus `json:"status"`
}

func updateFabricLoadBalancer(c client, ns, name, newHostName string) error {
	patchStatus := patchFabricStatus{
		Status: fabricStatus{
			LoadBalancer: fabricLoadBalancerStatus{
				Fabric: []fabricLoadBalancer{{Hostname: newHostName}},
			},
		},
	}

	resource := fmt.Sprintf(fabricPatchStatusResource, ns, name)
	payload, err := json.Marshal(patchStatus)
	if err != nil {
		return err
	}

	r, err := c.patch(resource, payload)
	if err != nil {
		return fmt.Errorf("failed to patch fabric %s/%s = %q: %v", ns, name, newHostName, err)
	}
	defer r.Close()
	return nil
}
