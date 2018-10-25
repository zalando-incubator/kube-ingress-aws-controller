package aws

import (
	"encoding/json"
	"fmt"
	"time"

	"crypto/sha256"
	"sort"

	"github.com/mweagle/go-cloudformation"
)

func hashARNs(certARNs []string) []byte {
	hash := sha256.New()

	for _, arn := range certARNs {
		hash.Write([]byte(arn))
		hash.Write([]byte{'\000'})
	}

	return hash.Sum(nil)
}

func generateTemplate(certs map[string]time.Time, idleConnectionTimeoutSeconds uint, albLogsS3Bucket, albLogsS3Prefix string) (string, error) {
	template := cloudformation.NewTemplate()
	template.Description = "Load Balancer for Kubernetes Ingress"
	template.Parameters = map[string]*cloudformation.Parameter{
		parameterLoadBalancerSchemeParameter: &cloudformation.Parameter{
			Type:        "String",
			Description: "The Load Balancer scheme - 'internal' or 'internet-facing'",
			Default:     "internet-facing",
		},
		parameterLoadBalancerSecurityGroupParameter: &cloudformation.Parameter{
			Type:        "List<AWS::EC2::SecurityGroup::Id>",
			Description: "The security group ID for the Load Balancer",
		},
		parameterLoadBalancerSubnetsParameter: &cloudformation.Parameter{
			Type:        "List<AWS::EC2::Subnet::Id>",
			Description: "The list of subnets IDs for the Load Balancer",
		},
		parameterTargetGroupHealthCheckPathParameter: &cloudformation.Parameter{
			Type:        "String",
			Description: "The healthcheck path",
			Default:     "/kube-system/healthz",
		},
		parameterTargetGroupHealthCheckPortParameter: &cloudformation.Parameter{
			Type:        "Number",
			Description: "The healthcheck port",
			Default:     "9999",
		},
		parameterTargetTargetPortParameter: &cloudformation.Parameter{
			Type:        "Number",
			Description: "The target port",
			Default:     "9999",
		},
		parameterTargetGroupHealthCheckIntervalParameter: &cloudformation.Parameter{
			Type:        "Number",
			Description: "The healthcheck interval",
			Default:     "10",
		},
		parameterTargetGroupVPCIDParameter: &cloudformation.Parameter{
			Type:        "AWS::EC2::VPC::Id",
			Description: "The VPCID for the TargetGroup",
		},
		parameterListenerSslPolicyParameter: &cloudformation.Parameter{
			Type:        "String",
			Description: "The HTTPS SSL Security Policy Name",
			Default:     "ELBSecurityPolicy-2016-08",
		},
		parameterIpAddressTypeParameter: &cloudformation.Parameter{
			Type:        "String",
			Description: "IP Address Type, 'ipv4' or 'dualstack'",
			Default:     "ipv4",
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
		// Sort the certificate names so we have a stable order.
		certificateARNs := make([]string, 0, len(certs))
		for certARN := range certs {
			certificateARNs = append(certificateARNs, certARN)
		}
		sort.Slice(certificateARNs, func(i, j int) bool {
			return certificateARNs[i] < certificateARNs[j]
		})

		// Add an HTTPS Listener resource with the first certificate as the default one
		template.AddResource("HTTPSListener", &cloudformation.ElasticLoadBalancingV2Listener{
			DefaultActions: &cloudformation.ElasticLoadBalancingV2ListenerActionList{
				{
					Type:           cloudformation.String("forward"),
					TargetGroupArn: cloudformation.Ref("TG").String(),
				},
			},
			Certificates: &cloudformation.ElasticLoadBalancingV2ListenerCertificatePropertyList{
				{
					CertificateArn: cloudformation.String(certificateARNs[0]),
				},
			},
			LoadBalancerArn: cloudformation.Ref("LB").String(),
			Port:            cloudformation.Integer(443),
			Protocol:        cloudformation.String("HTTPS"),
			SslPolicy:       cloudformation.Ref(parameterListenerSslPolicyParameter).String(),
		})

		// Add a ListenerCertificate resource with all of the certificates, including the default one
		certificateList := make(cloudformation.ElasticLoadBalancingV2ListenerCertificateCertificateList, 0, len(certificateARNs))
		for _, certARN := range certificateARNs {
			c := cloudformation.ElasticLoadBalancingV2ListenerCertificateCertificate{
				CertificateArn: cloudformation.String(certARN),
			}
			certificateList = append(certificateList, c)
		}

		// Use a new resource name every time to avoid a bug where CloudFormation fails to perform an update properly
		resourceName := fmt.Sprintf("HTTPSListenerCertificate%x", hashARNs(certificateARNs))
		template.AddResource(resourceName, &cloudformation.ElasticLoadBalancingV2ListenerCertificate{
			Certificates: &certificateList,
			ListenerArn:  cloudformation.Ref("HTTPSListener").String(),
		})
	}

	// Build up the LoadBalancerAttributes list, as there is no way to make attributes conditional in the template
	albAttrList := make(cloudformation.ElasticLoadBalancingV2LoadBalancerLoadBalancerAttributeList, 0, 4)
	albAttrList = append(albAttrList,
		cloudformation.ElasticLoadBalancingV2LoadBalancerLoadBalancerAttribute{
			Key:   cloudformation.String("idle_timeout.timeout_seconds"),
			Value: cloudformation.String(fmt.Sprintf("%d", idleConnectionTimeoutSeconds)),
		},
	)
	if albLogsS3Bucket != "" {
		albAttrList = append(albAttrList,
			cloudformation.ElasticLoadBalancingV2LoadBalancerLoadBalancerAttribute{
				Key:   cloudformation.String("access_logs.s3.enabled"),
				Value: cloudformation.String("true"),
			},
		)
		albAttrList = append(albAttrList,
			cloudformation.ElasticLoadBalancingV2LoadBalancerLoadBalancerAttribute{
				Key:   cloudformation.String("access_logs.s3.bucket"),
				Value: cloudformation.String(albLogsS3Bucket),
			},
		)
		if albLogsS3Prefix != "" {
			albAttrList = append(albAttrList,
				cloudformation.ElasticLoadBalancingV2LoadBalancerLoadBalancerAttribute{
					Key:   cloudformation.String("access_logs.s3.prefix"),
					Value: cloudformation.String(albLogsS3Prefix),
				},
			)
		}
	} else {
		albAttrList = append(albAttrList,
			cloudformation.ElasticLoadBalancingV2LoadBalancerLoadBalancerAttribute{
				Key:   cloudformation.String("access_logs.s3.enabled"),
				Value: cloudformation.String("false"),
			},
		)
	}

	template.AddResource("LB", &cloudformation.ElasticLoadBalancingV2LoadBalancer{
		LoadBalancerAttributes: &albAttrList,

		IPAddressType:  cloudformation.Ref(parameterIpAddressTypeParameter).String(),
		Scheme:         cloudformation.Ref(parameterLoadBalancerSchemeParameter).String(),
		SecurityGroups: cloudformation.Ref(parameterLoadBalancerSecurityGroupParameter).StringList(),
		Subnets:        cloudformation.Ref(parameterLoadBalancerSubnetsParameter).StringList(),
		Tags: &cloudformation.TagList{
			{
				Key:   cloudformation.String("StackName"),
				Value: cloudformation.Ref("AWS::StackName").String(),
			},
		},
	})
	template.AddResource("TG", &cloudformation.ElasticLoadBalancingV2TargetGroup{
		HealthCheckIntervalSeconds: cloudformation.Ref(parameterTargetGroupHealthCheckIntervalParameter).Integer(),
		HealthCheckPath:            cloudformation.Ref(parameterTargetGroupHealthCheckPathParameter).String(),
		HealthCheckPort:            cloudformation.Ref(parameterTargetGroupHealthCheckPortParameter).String(),
		Port:                       cloudformation.Ref(parameterTargetTargetPortParameter).Integer(),
		Protocol:                   cloudformation.String("HTTP"),
		VPCID:                      cloudformation.Ref(parameterTargetGroupVPCIDParameter).String(),
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
