package aws

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mweagle/go-cloudformation"
)

func generateTemplate(certs map[string]time.Time, hostnames map[string]struct{}) (string, error) {
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
	}
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
		certificateARNs := make(cloudformation.ElasticLoadBalancingV2ListenerCertificateCertificateList, 0, len(certs)-1)
		first := true
		for certARN, _ := range certs {
			if first {
				defaultCertificateARN := cloudformation.ElasticLoadBalancingV2ListenerCertificatePropertyList{
					{
						CertificateArn: cloudformation.String(certARN),
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

				first = false
				continue
			}

			c := cloudformation.ElasticLoadBalancingV2ListenerCertificateCertificate{
				CertificateArn: cloudformation.String(certARN),
			}
			certificateARNs = append(certificateARNs, c)
		}

		if len(certificateARNs) > 0 {
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
	// default Target group
	template.AddResource("TG", &cloudformation.ElasticLoadBalancingV2TargetGroup{
		HealthCheckIntervalSeconds: cloudformation.Ref("TargetGroupHealthCheckIntervalParameter").Integer(),
		HealthCheckPath:            cloudformation.Ref("TargetGroupHealthCheckPathParameter").String(),
		Port:                       cloudformation.Ref("TargetGroupHealthCheckPortParameter").Integer(),
		Protocol:                   cloudformation.String("HTTP"),
		VPCID:                      cloudformation.Ref("TargetGroupVPCIDParameter").String(),
	})

	targetGroupARNs := make([]cloudformation.Stringable, 0, len(hostnames)+1)
	targetGroupARNs = append(targetGroupARNs, cloudformation.Ref("TG").String())

	// add target group per hostname
	i := int64(1)
	for hostname := range hostnames {
		h := sha1.New()
		h.Write([]byte(hostname))
		resourceHash := fmt.Sprintf("%x", h.Sum(nil))

		template.AddResource(resourceHash+"TG", &cloudformation.ElasticLoadBalancingV2TargetGroup{
			HealthCheckIntervalSeconds: cloudformation.Ref("TargetGroupHealthCheckIntervalParameter").Integer(),
			HealthCheckPath:            cloudformation.Ref("TargetGroupHealthCheckPathParameter").String(),
			Port:                       cloudformation.Ref("TargetGroupHealthCheckPortParameter").Integer(),
			Protocol:                   cloudformation.String("HTTP"),
			VPCID:                      cloudformation.Ref("TargetGroupVPCIDParameter").String(),
		})

		targetGroupARNs = append(targetGroupARNs, cloudformation.Ref(resourceHash+"TG").String())

		template.AddResource(resourceHash+"LR", &cloudformation.ElasticLoadBalancingV2ListenerRule{
			Actions: &cloudformation.ElasticLoadBalancingV2ListenerRuleActionList{
				{
					Type:           cloudformation.String("forward"),
					TargetGroupArn: cloudformation.Ref(resourceHash + "TG").String(),
				},
			},
			Conditions: &cloudformation.ElasticLoadBalancingV2ListenerRuleRuleConditionList{
				{
					Field:  cloudformation.String("host-header"),
					Values: cloudformation.StringList(cloudformation.String(hostname)),
				},
			},
			ListenerArn: cloudformation.Ref("HTTPSListener").String(),
			Priority:    cloudformation.Integer(i),
		})

		i++
	}

	template.Outputs = map[string]*cloudformation.Output{
		"LoadBalancerDNSName": &cloudformation.Output{
			Description: "DNS name for the LoadBalancer",
			Value:       cloudformation.GetAtt("LB", "DNSName").String(),
		},
		"TargetGroupARNs": &cloudformation.Output{
			Description: "The ARN of the TargetGroups",
			Value:       cloudformation.Join(",", targetGroupARNs...),
		},
	}

	stackTemplate, err := json.MarshalIndent(template, "", "    ")
	if err != nil {
		return "", err
	}

	return string(stackTemplate), nil
}
