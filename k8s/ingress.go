package k8s

import "time"

type IngressList struct {
	Kind       string              `json:"kind"`
	ApiVersion string              `json:"apiVersion"`
	Metadata   ingressListMetadata `json:"metadata"`
	Items      []Ingress           `json:"items"`
}

type ListMeta struct {
	Namespace   string                 `json:"namespace"`
	Name        string                 `json:"name"`
	UID         string                 `json:"uid"`
	Annotations map[string]interface{} `json:"annotations"`
}

type ingressListMetadata struct {
	SelfLink        string `json:"selfLink"`
	ResourceVersion string `json:"resourceVersion"`
}

type Ingress struct {
	Metadata ListMeta               `json:"metadata"`
	Spec     IngressSpec            `json:"spec"`
	Status   map[string]interface{} `json:"status"`
}

type ingressItemMetadata struct {
	ListMeta
	SelfLink          string                 `json:"selfLink"`
	ResourceVersion   string                 `json:"resourceVersion"`
	Generation        int                    `json:"generation"`
	CreationTimestamp time.Time              `json:"creationTimestamp"`
	DeletionTimestamp time.Time              `json:"deletionTimestamp"`
	Labels            map[string]interface{} `json:"labels"`
}

type IngressSpec struct {
	Rules []ingressItemRule `json:"rules"`
}

type ingressItemRule struct {
	Host string `json:"host"`
}

const (
	ingressListResource = "/apis/extensions/v1beta1/ingresses"
)
