package aws

import (
	"encoding/json"

	"github.com/mweagle/go-cloudformation"
)

func generateTemplate(certs []string) (string, error) {
	template := cloudformation.NewTemplate()
	template.Description = "Load Balancer for Kubernetes Ingress"
	template.Parameters = map[string]*cloudformation.Parameter{
		"LoadBalancerSchemeParameter": &cloudformation.Parameter{
			Type:        "String",
			Description: "The Load Balancer scheme - 'internal' or 'internet-facing'",
			Default:     "internet-facing",
		},
		"LoadBalancerSecurityGroupParameter": &cloudformation.Parameter{
			Type:        "List<AWS::EC2::SecurityGroup::Id>",
			Description: "The security group ID for the Load Balancer",
		},
		"LoadBalancerSubnetsParameter": &cloudformation.Parameter{
			Type:        "List<AWS::EC2::Subnet::Id>",
			Description: "The list of subnets IDs for the Load Balancer",
		},
		"TargetGroupHealthCheckPathParameter": &cloudformation.Parameter{
			Type:        "String",
			Description: "The healthcheck path",
			Default:     "/kube-system/healthz",
		},
		"TargetGroupHealthCheckPortParameter": &cloudformation.Parameter{
			Type:        "Number",
			Description: "The healthcheck port",
			Default:     "9999",
		},
		"TargetGroupHealthCheckIntervalParameter": &cloudformation.Parameter{
			Type:        "Number",
			Description: "The healthcheck interval",
			Default:     "10",
		},
		"TargetGroupVPCIDParameter": &cloudformation.Parameter{
			Type:        "AWS::EC2::VPC::Id",
			Description: "The VPCID for the TargetGroup",
		},
		// "ListenerCertificatesParameter": &cloudformation.Parameter{
		// 	Type:        "String",
		// 	Description: "The HTTPS Listener certificate ARNs (IAM/ACM)",
		// 	Default:     "",
		// },
	}
	// template.Conditions = map[string]interface{}{
	// 	"": "",
	// }
	template.AddResource("HTTPListener", &cloudformation.ElasticLoadBalancingV2Listener{
		DefaultActions: &cloudformation.ElasticLoadBalancingV2ListenerActionList{
			{
				Type:           cloudformation.String("forward"),
				TargetGroupArn: cloudformation.Ref("TG").String(),
			},
		},
		LoadBalancerArn: cloudformation.Ref("LB").String(),
		Port:            cloudformation.Integer(80),
		Protocol:        cloudformation.String("HTTP"),
	})

	if len(certs) > 0 {
		defaultCertificateARN := cloudformation.ElasticLoadBalancingV2ListenerCertificatePropertyList{
			{
				CertificateArn: cloudformation.String(certs[0]),
			},
		}

		template.AddResource("HTTPSListener", &cloudformation.ElasticLoadBalancingV2Listener{
			DefaultActions: &cloudformation.ElasticLoadBalancingV2ListenerActionList{
				{
					Type:           cloudformation.String("forward"),
					TargetGroupArn: cloudformation.Ref("TG").String(),
				},
			},
			Certificates:    &defaultCertificateARN,
			LoadBalancerArn: cloudformation.Ref("LB").String(),
			Port:            cloudformation.Integer(443),
			Protocol:        cloudformation.String("HTTPS"),
		})

		if len(certs) > 1 {
			certificateARNs := make(cloudformation.ElasticLoadBalancingV2ListenerCertificateCertificateList, 0, len(certs)-1)
			for _, cert := range certs[1:] {
				c := cloudformation.ElasticLoadBalancingV2ListenerCertificateCertificate{
					CertificateArn: cloudformation.String(cert),
				}
				certificateARNs = append(certificateARNs, c)
			}

			template.AddResource("HTTPSListenerCertificate", &cloudformation.ElasticLoadBalancingV2ListenerCertificate{
				Certificates: &certificateARNs,
				ListenerArn:  cloudformation.Ref("HTTPSListener").String(),
			})
		}
	}
	template.AddResource("LB", &cloudformation.ElasticLoadBalancingV2LoadBalancer{
		Scheme:         cloudformation.Ref("LoadBalancerSchemeParameter").String(),
		SecurityGroups: cloudformation.Ref("LoadBalancerSecurityGroupParameter").StringList(),
		Subnets:        cloudformation.Ref("LoadBalancerSubnetsParameter").StringList(),
		Tags: &cloudformation.TagList{
			{
				Key:   cloudformation.String("StackName"),
				Value: cloudformation.Ref("AWS::StackName").String(),
			},
		},
	})
	template.AddResource("TG", &cloudformation.ElasticLoadBalancingV2TargetGroup{
		HealthCheckIntervalSeconds: cloudformation.Ref("TargetGroupHealthCheckIntervalParameter").Integer(),
		HealthCheckPath:            cloudformation.Ref("TargetGroupHealthCheckPathParameter").String(),
		Port:                       cloudformation.Ref("TargetGroupHealthCheckPortParameter").Integer(),
		Protocol:                   cloudformation.String("HTTP"),
		VPCID:                      cloudformation.Ref("TargetGroupVPCIDParameter").String(),
	})

	template.Outputs = map[string]*cloudformation.Output{
		"LoadBalancerDNSName": &cloudformation.Output{
			Description: "DNS name for the LoadBalancer",
			Value:       cloudformation.GetAtt("LB", "DNSName").String(),
		},
		"TargetGroupARN": &cloudformation.Output{
			Description: "The ARN of the TargetGroup",
			Value:       cloudformation.Ref("TG").String(),
		},
	}

	stackTemplate, err := json.MarshalIndent(template, "", "    ")
	if err != nil {
		return "", err
	}

	return string(stackTemplate), nil
}
