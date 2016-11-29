package k8s

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
)

func List() (*IngressList, error) {
	kc, err := defaultKubeClient()
	if err != nil {
		return nil, err
	}

	r, err := kc.Get(ingressListResource)
	if err != nil {
		return nil, fmt.Errorf("failed to get ingress list: %v", err)
	}

	defer r.Close()

	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	var result IngressList
	if err := json.Unmarshal(b, &result); err != nil {
		return nil, err
	}

	return &result, nil
}
