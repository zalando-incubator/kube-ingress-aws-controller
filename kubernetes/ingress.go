package kubernetes

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"time"
)

// IngressList is used to deserialize Kubernete's API resources with the same name.
type IngressList struct {
	Kind       string              `json:"kind"`
	APIVersion string              `json:"apiVersion"`
	Metadata   ingressListMetadata `json:"metadata"`
	Items      []Ingress           `json:"items"`
}

// ListMeta is used to deserialize Kubernete's API resources with the same name.
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

// Ingress is used to deserialize Kubernete's API resources with the same name.
type Ingress struct {
	Metadata listMeta      `json:"metadata"`
	Spec     ingressSpec   `json:"spec"`
	Status   ingressStatus `json:"status"`
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

type ingressStatus struct {
	LoadBalancer loadBalancerStatus `json:"loadBalancer"`
}

type loadBalancerStatus struct {
	Ingress []loadBalancerIngress `json:"ingress"`
}

type loadBalancerIngress struct {
	Hostname string `json:"hostname"`
}

const (
	ingressListResource             = "/apis/extensions/v1beta1/ingresses"
	ingressPatchStatusResource      = "/apis/extensions/v1beta1/namespaces/%s/ingresses/%s/status"
	ingressCertificateARNAnnotation = "zalando.org/aws-load-balancer-ssl-cert"
)

// CertificateARN returns the AWS certificate (IAM or ACM) ARN found in the ingress resource metadata.
// It returns an empty string if the annotation is missing.
func (i *Ingress) CertificateARN() string {
	return i.getMetadataString(ingressCertificateARNAnnotation, "")
}

// String returns a string representation of the Ingress resource.
func (i Ingress) String() string {
	return fmt.Sprintf("%s/%s", i.Metadata.Namespace, i.Metadata.Name)
}

// Hostname returns the DNS hostname already associated with the ingress. If not currently associated with any
// load balancer the result is an empty string
func (i *Ingress) Hostname() string {
	if len(i.Status.LoadBalancer.Ingress) < 1 {
		return ""
	}
	return i.Status.LoadBalancer.Ingress[0].Hostname
}

func (i *Ingress) getMetadataString(key string, defaultValue string) string {
	if val, ok := i.Metadata.Annotations[key].(string); ok {
		return val
	}
	return defaultValue
}

// ListIngress can be used to obtain the list of ingress resources for all namespaces.
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

type patchIngressStatus struct {
	Status ingressStatus `json:"status"`
}

// UpdateIngressLoaBalancer can be used to update the loadBalancer object of an ingress resource using the lb DNS name.
func UpdateIngressLoaBalancer(client *Client, ingresses []Ingress, loadBalancerDNSName string) []error {
	patchStatus := patchIngressStatus{
		Status: ingressStatus{
			LoadBalancer: loadBalancerStatus{
				Ingress: []loadBalancerIngress{
					{Hostname: loadBalancerDNSName},
				},
			},
		},
	}

	var errors []error
	for _, ingress := range ingresses {
		ns, name := ingress.Metadata.Namespace, ingress.Metadata.Name
		hostname := ingress.Hostname()
		if hostname == loadBalancerDNSName {
			log.Printf("ingress %v already has hostname %q. skipping update", ingress, hostname)
			continue
		}
		resource := fmt.Sprintf(ingressPatchStatusResource, ns, name)
		payload, err := json.Marshal(patchStatus)
		if err != nil {
			return []error{err}
		}

		r, err := client.Patch(resource, payload)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to patch ingress %s/%s = %q. %v", ns, name, hostname, err))
		} else {
			log.Printf("updated ingress %v with DNS name %q\n", ingress, loadBalancerDNSName)
		}
		r.Close() // discard response
	}
	return errors
}
