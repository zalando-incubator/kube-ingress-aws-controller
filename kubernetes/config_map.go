package kubernetes

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
)

const (
	configMapResource = "/api/v1/namespaces/%s/configmaps/%s"
)

type configMapMetadata struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type configMap struct {
	Kind       string            `json:"kind"`
	APIVersion string            `json:"apiVersion"`
	Metadata   configMapMetadata `json:"metadata"`
	Data       map[string]string `json:"data"`
}

func getConfigMap(c client, namespace, name string) (*configMap, error) {
	resource := fmt.Sprintf(configMapResource, namespace, name)

	r, err := c.get(resource)
	if err != nil {
		return nil, fmt.Errorf("failed to get ConfigMap %s/%s: %v", namespace, name, err)
	}

	defer r.Close()

	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read ConfigMap %s/%s: %v", namespace, name, err)
	}

	var result configMap
	if err := json.Unmarshal(b, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ConfigMap %s/%s: %v", namespace, name, err)
	}

	return &result, nil
}
