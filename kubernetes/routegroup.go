package kubernetes

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"time"
)

type routegroupList struct {
	Kind       string                 `json:"kind"`
	APIVersion string                 `json:"apiVersion"`
	Metadata   routegroupListMetadata `json:"metadata"`
	Items      []*routegroup          `json:"items"`
}

type routegroup struct {
	Metadata routegroupItemMetadata `json:"metadata"`
	Spec     routegroupSpec         `json:"spec"`
	Status   routegroupStatus       `json:"status"`
}

type routegroupListMetadata struct {
	SelfLink        string `json:"selfLink"`
	ResourceVersion string `json:"resourceVersion"`
}

type routegroupItemMetadata struct {
	Namespace         string                 `json:"namespace"`
	Name              string                 `json:"name"`
	UID               string                 `json:"uid"`
	Annotations       map[string]interface{} `json:"annotations"`
	SelfLink          string                 `json:"selfLink"`
	ResourceVersion   string                 `json:"resourceVersion"`
	Generation        int                    `json:"generation"`
	CreationTimestamp time.Time              `json:"creationTimestamp"`
	DeletionTimestamp time.Time              `json:"deletionTimestamp"`
	Labels            map[string]interface{} `json:"labels"`
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

func (rg *routegroup) getAnnotationsString(key string, defaultValue string) string {
	if val, ok := rg.Metadata.Annotations[key].(string); ok {
		return val
	}
	return defaultValue
}

func listRoutegroups(c client) (*routegroupList, error) {
	r, err := c.get(routegroupListResource)
	if err != nil {
		return nil, fmt.Errorf("failed to get routegroup list: %v", err)
	}

	defer r.Close()

	b, err := ioutil.ReadAll(r)
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

func updateRoutegroupLoadBalancer(c client, rg *routegroup, newHostName string) error {
	ns, name := rg.Metadata.Namespace, rg.Metadata.Name
	for _, routegroupLb := range rg.Status.LoadBalancer.Routegroup {
		if routegroupLb.Hostname == newHostName {
			return ErrUpdateNotNeeded
		}
	}

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
		return fmt.Errorf("failed to patch routegroup %s/%s = %q: %v", ns, name, newHostName, err)
	}
	defer r.Close()
	return nil
}
