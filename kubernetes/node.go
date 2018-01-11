package kubernetes

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
)

type nodeList struct {
	Kind       string  `json:"kind"`
	APIVersion string  `json:"apiVersion"`
	Items      []*node `json:"items"`
}

type node struct {
	Metadata nodeMetadata `json:"metadata"`
	Spec     nodeSpec     `json:"spec"`
	Status   nodeStatus   `json:"status"`
}

type nodeMetadata struct {
	Name            string                 `json:"name"`
	Uid             string                 `json:"uid"`
	Annotations     map[string]interface{} `json:"annotations"`
	SelfLink        string                 `json:"selfLink"`
	ResourceVersion string                 `json:"resourceVersion"`
	Labels          map[string]interface{} `json:"labels"`
}

type nodeSpec struct {
	ExternalId string `json:"externalID"`
	PodCIDR    string `json:"podCIDR"`
	ProviderId string `json:"providerID"`
}

type nodeStatus struct {
	Addresses []nodeAddress `json:"addresses"`
}

type nodeAddress struct {
	Address string `json:"address"`
	Type    string `json:"type"`
}

const (
	nodeAddressTypeInternalIP = "InternalIP"
	nodeAddressTypeHostname   = "Hostname"
	nodeListResource          = "/api/v1/nodes"
	nodeRoleLabelName         = "kubernetes.io/role"
	nodeRoleLabelValueMaster  = "master"
	nodeRoleLabelValueNode    = "node"
)

func (i *node) getAddress(addrType string) string {
	for _, addr := range i.Status.Addresses {
		if addr.Type == addrType {
			return addr.Address
		}
	}
	return ""
}

func listNode(c client) (*nodeList, error) {
	r, err := c.get(nodeListResource)
	if err != nil {
		return nil, fmt.Errorf("failed to get node list: %v", err)
	}

	defer r.Close()

	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	var result nodeList
	if err := json.Unmarshal(b, &result); err != nil {
		return nil, err
	}

	return &result, nil
}
