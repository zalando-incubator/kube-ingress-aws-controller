// This program emits a cloudformation document for `app` to stdout
package main

import (
	"encoding/json"
	"log"
	"os"

	cf "github.com/zalando-incubator/kube-ingress-aws-controller/internal/aws/cloudformation"
)

func makeTemplate() *cf.Template {
	t := cf.NewTemplate()
	t.Description = "example production infrastructure"
	t.Parameters["DnsName"] = &cf.Parameter{
		Description: "The top level DNS name for the infrastructure",
		Type:        "String",
		Default:     "preview.example.io",
	}

	t.AddResource("ServerLoadBalancer", cf.ElasticLoadBalancingLoadBalancer{
		ConnectionDrainingPolicy: &cf.ElasticLoadBalancingLoadBalancerConnectionDrainingPolicy{
			Enabled: cf.Bool(true),
			Timeout: cf.Integer(30),
		},
		CrossZone: cf.Bool(true),
		HealthCheck: &cf.ElasticLoadBalancingLoadBalancerHealthCheck{
			HealthyThreshold:   cf.String("2"),
			Interval:           cf.String("60"),
			Target:             cf.String("HTTP:80/"),
			Timeout:            cf.String("5"),
			UnhealthyThreshold: cf.String("2"),
		},
		Listeners: &cf.ElasticLoadBalancingLoadBalancerListenersList{
			cf.ElasticLoadBalancingLoadBalancerListeners{
				InstancePort:     cf.String("8000"),
				InstanceProtocol: cf.String("TCP"),
				LoadBalancerPort: cf.String("443"),
				Protocol:         cf.String("SSL"),
				SSLCertificateID: cf.Join("",
					cf.String("arn:aws:iam::"),
					cf.Ref("AWS::AccountID"),
					cf.String(":server-certificate/"),
					cf.Ref("DnsName")),
			},
		},
		Policies: &cf.ElasticLoadBalancingLoadBalancerPoliciesList{
			cf.ElasticLoadBalancingLoadBalancerPolicies{
				PolicyName: cf.String("EnableProxyProtocol"),
				PolicyType: cf.String("ProxyProtocolPolicyType"),
				Attributes: []interface{}{
					map[string]interface{}{
						"Name":  "ProxyProtocol",
						"Value": "true",
					},
				},
				InstancePorts: cf.StringList(cf.String("8000")),
			},
		},
		Subnets: cf.StringList(
			cf.Ref("VpcSubnetA"),
			cf.Ref("VpcSubnetB"),
			cf.Ref("VpcSubnetC"),
		),
		SecurityGroups: cf.StringList(cf.Ref("LoadBalancerSecurityGroup")),
	})

	return t
}

func main() {
	template := makeTemplate()
	buf, err := json.MarshalIndent(template, "", "  ")
	if err != nil {
		log.Fatalf("marshal: %s", err)
	}
	os.Stdout.Write(buf)
}
