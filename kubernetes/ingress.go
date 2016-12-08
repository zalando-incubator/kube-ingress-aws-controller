package kubernetes

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/zalando-incubator/kube-ingress-aws-controller/aws"
)

// IngressList is used to deserialize Kubernete's API resources with the same name
type IngressList struct {
	Kind       string              `json:"kind"`
	APIVersion string              `json:"apiVersion"`
	Metadata   ingressListMetadata `json:"metadata"`
	Items      []Ingress           `json:"items"`
}

// ListMeta is used to deserialize Kubernete's API resources with the same name
type listMeta struct {
	Namespace   string                 `json:"namespace"`
	Name        string                 `json:"name"`
	UID         string                 `json:"uid"`
	Annotations map[string]interface{} `json:"annotations"`
}

type ingressListMetadata struct {
	SelfLink        string `json:"selfLink"`
	ResourceVersion string `json:"resourceVersion"`
}

// Ingress is used to deserialize Kubernete's API resources with the same name
type Ingress struct {
	Metadata listMeta               `json:"metadata"`
	Spec     ingressSpec            `json:"spec"`
	Status   map[string]interface{} `json:"status"`
}

type ingressItemMetadata struct {
	listMeta
	SelfLink          string                 `json:"selfLink"`
	ResourceVersion   string                 `json:"resourceVersion"`
	Generation        int                    `json:"generation"`
	CreationTimestamp time.Time              `json:"creationTimestamp"`
	DeletionTimestamp time.Time              `json:"deletionTimestamp"`
	Labels            map[string]interface{} `json:"labels"`
}

type ingressSpec struct {
	Rules []ingressItemRule `json:"rules"`
}

type ingressItemRule struct {
	Host string `json:"host"`
}

const (
	ingressListResource             = "/apis/extensions/v1beta1/ingresses"
	ingressPatchStatusResource      = "/apis/extensions/v1beta1/namespaces/%s/ingresses/%s/status"
	ingressCertificateARNAnnotation = "zalando.org/aws-load-balancer-ssl-cert"
)

// CertificateARN returns the AWS certificate (IAM or ACM) ARN found in the ingress resource metadata.
// It returns an empty string if the annotation is missing
func (i *Ingress) CertificateARN() string {
	return i.getMetadataString(ingressCertificateARNAnnotation, "")
}

// String returns a string representation of the Ingress resource
func (i Ingress) String() string {
	return fmt.Sprintf("%s/%s", i.Metadata.Namespace, i.Metadata.Name)
}

func (i *Ingress) getMetadataString(key string, defaultValue string) string {
	val, has := i.Metadata.Annotations[key]
	if !has {
		return defaultValue
	}
	ret, ok := val.(string)
	if !ok {
		return defaultValue
	}
	return ret
}

const patchIngressesPayloadTemplate = `{"status":{"loadBalancer":{"ingress":[{"hostname":"%s"}]}}}`

// ListIngress can be used to obtain the list of ingress resources for all namespaces
func ListIngress(client *Client) (*IngressList, error) {
	r, err := client.Get(ingressListResource)
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

// UpdateIngressLoaBalancer can be used to update the loadBalancer object of an ingress resource using the lb DNS name
func UpdateIngressLoaBalancer(client *Client, ingresses []Ingress, lb *aws.LoadBalancer) error {
	payload := fmt.Sprintf(patchIngressesPayloadTemplate, lb.DNSName())
	for _, ingress := range ingresses {
		resource := fmt.Sprintf(ingressPatchStatusResource, ingress.Metadata.Namespace, ingress.Metadata.Name)
		r, err := client.Patch(resource, payload)
		if err != nil {
			return fmt.Errorf("failed to patch ingress %s/%s. %v", ingress.Metadata.Namespace, ingress.Metadata.Name, err)
		}
		r.Close() // discard response
	}
	return nil
}
