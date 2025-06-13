package aws

import (
	"encoding/json"
	"fmt"
	"strings"

	"crypto/sha256"
	"sort"

	cloudformation "github.com/mweagle/go-cloudformation"
)

const (
	// https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-rulecondition.html#cfn-elasticloadbalancingv2-listenerrule-rulecondition-field
	listenerRuleConditionHostField = "host-header"

	// https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-action.html#cfn-elasticloadbalancingv2-listenerrule-action-type
	listenerRuleActionTypeFixedRes = "fixed-response"

	// internalTrafficDenyRulePriority defines the [priority][0] for the
	// rule that denies requests directed to internal domains on the HTTP
	// and HTTPS listener.
	//
	// [0]: https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-listenerrule.html#cfn-elasticloadbalancingv2-listenerrule-priority
	internalTrafficDenyRulePriority int64 = 1

	loadBalancerResourceLogicalID = "LB"
)

func hashARNs(certARNs []string) []byte {
	hash := sha256.New()

	for _, arn := range certARNs {
		hash.Write([]byte(arn))
		hash.Write([]byte{'\000'})
	}

	return hash.Sum(nil)
}

func generateTemplate(spec *stackSpec) (string, error) {
	template := cloudformation.NewTemplate()
	template.Description = "Load Balancer for Kubernetes Ingress"
	template.Parameters = map[string]*cloudformation.Parameter{
		parameterLoadBalancerSchemeParameter: {
			Type:        "String",
			Description: "The Load Balancer scheme - 'internal' or 'internet-facing'",
			Default:     "internet-facing",
		},
		parameterLoadBalancerSecurityGroupParameter: {
			Type:        "List<AWS::EC2::SecurityGroup::Id>",
			Description: "The security group ID for the Load Balancer",
		},
		parameterLoadBalancerSubnetsParameter: {
			Type:        "List<AWS::EC2::Subnet::Id>",
			Description: "The list of subnets IDs for the Load Balancer",
		},
		parameterTargetGroupHealthCheckPathParameter: {
			Type:        "String",
			Description: "The healthcheck path",
			Default:     "/kube-system/healthz",
		},
		parameterTargetGroupHealthCheckPortParameter: {
			Type:        "Number",
			Description: "The healthcheck port",
			Default:     "9999",
		},
		parameterTargetGroupTargetPortParameter: {
			Type:        "Number",
			Description: "The target port",
			Default:     "9999",
		},
		parameterTargetGroupHealthCheckIntervalParameter: {
			Type:        "Number",
			Description: "The healthcheck interval",
			Default:     "10",
		},
		parameterTargetGroupHealthCheckTimeoutParameter: {
			Type:        "Number",
			Description: "The healthcheck timeout",
			Default:     "5",
		},
		parameterTargetGroupVPCIDParameter: {
			Type:        "AWS::EC2::VPC::Id",
			Description: "The VPCID for the TargetGroup",
		},
		parameterListenerSslPolicyParameter: {
			Type:        "String",
			Description: "The HTTPS SSL Security Policy Name",
			Default:     "ELBSecurityPolicy-2016-08",
		},
		parameterIpAddressTypeParameter: {
			Type:        "String",
			Description: "IP Address Type, 'ipv4' or 'dualstack'",
			Default:     IPAddressTypeIPV4,
		},
		parameterLoadBalancerTypeParameter: {
			Type:        "String",
			Description: "Loadbalancer Type, 'application' or 'network'",
			Default:     LoadBalancerTypeApplication,
		},
		parameterHTTP2Parameter: {
			Type:        "String",
			Description: "H2 Enabled",
			Default:     "true",
		},
	}

	if spec.wafWebAclId != "" {
		template.Parameters[parameterLoadBalancerWAFWebACLIDParameter] = &cloudformation.Parameter{
			Type:        "String",
			Description: "Associated WAF ID or ARN.",
		}
	}

	const httpsTargetGroupName = "TG"

	template.Outputs = map[string]*cloudformation.Output{
		outputLoadBalancerDNSName: {
			Description: "DNS name for the LoadBalancer",
			Value:       cloudformation.GetAtt(loadBalancerResourceLogicalID, "DNSName").String(),
		},
		outputTargetGroupARN: {
			Description: "The ARN of the TargetGroup",
			Value:       cloudformation.Ref(httpsTargetGroupName).String(),
		},
	}

	template.AddResource(httpsTargetGroupName, newTargetGroup(spec, parameterTargetGroupTargetPortParameter))

	if !spec.httpDisabled {
		// Use the same target group for HTTP Listener or create another one if needed
		httpTargetGroupName := httpsTargetGroupName
		if spec.httpTargetPort != spec.targetPort {
			httpTargetGroupName = "TGHTTP"
			template.Parameters[parameterTargetGroupHTTPTargetPortParameter] = &cloudformation.Parameter{
				Type:        "Number",
				Description: "The HTTP target port",
			}
			template.Outputs[outputHTTPTargetGroupARN] = &cloudformation.Output{
				Description: "The ARN of the HTTP TargetGroup",
				Value:       cloudformation.Ref(httpTargetGroupName).String(),
			}
			template.AddResource(httpTargetGroupName, newTargetGroup(spec, parameterTargetGroupHTTPTargetPortParameter))
		}

		// Add an HTTP Listener resource
		if spec.loadbalancerType == LoadBalancerTypeApplication {
			if spec.httpRedirectToHTTPS {
				template.AddResource("HTTPListener", &cloudformation.ElasticLoadBalancingV2Listener{
					DefaultActions: &cloudformation.ElasticLoadBalancingV2ListenerActionList{
						{
							Type: cloudformation.String("redirect"),
							RedirectConfig: &cloudformation.ElasticLoadBalancingV2ListenerRedirectConfig{
								Protocol:   cloudformation.String("HTTPS"),
								Port:       cloudformation.String("443"),
								Host:       cloudformation.String("#{host}"),
								Path:       cloudformation.String("/#{path}"),
								Query:      cloudformation.String("#{query}"),
								StatusCode: cloudformation.String("HTTP_301"),
							},
						},
					},
					LoadBalancerArn: cloudformation.Ref(loadBalancerResourceLogicalID).String(),
					Port:            cloudformation.Integer(80),
					Protocol:        cloudformation.String("HTTP"),
				})
			} else {
				template.AddResource("HTTPListener", &cloudformation.ElasticLoadBalancingV2Listener{
					DefaultActions: &cloudformation.ElasticLoadBalancingV2ListenerActionList{
						{
							Type:           cloudformation.String("forward"),
							TargetGroupArn: cloudformation.Ref(httpTargetGroupName).String(),
						},
					},
					LoadBalancerArn: cloudformation.Ref(loadBalancerResourceLogicalID).String(),
					Port:            cloudformation.Integer(80),
					Protocol:        cloudformation.String("HTTP"),
				})
				if spec.denyInternalDomains {
					template.AddResource(
						"HTTPRuleBlockInternalTraffic",
						generateDenyInternalTrafficRule(
							"HTTPListener",
							internalTrafficDenyRulePriority,
							spec.internalDomains,
							spec.denyInternalDomainsResponse,
						),
					)
				}
			}
		} else if spec.loadbalancerType == LoadBalancerTypeNetwork {
			template.AddResource("HTTPListener", &cloudformation.ElasticLoadBalancingV2Listener{
				DefaultActions: &cloudformation.ElasticLoadBalancingV2ListenerActionList{
					{
						Type:           cloudformation.String("forward"),
						TargetGroupArn: cloudformation.Ref(httpTargetGroupName).String(),
					},
				},
				LoadBalancerArn: cloudformation.Ref(loadBalancerResourceLogicalID).String(),
				Port:            cloudformation.Integer(80),
				Protocol:        cloudformation.String("TCP"),
			})
		}
	}

	if len(spec.certificateARNs) > 0 {
		// Sort the certificate names so we have a stable order.
		certificateARNs := make([]string, 0, len(spec.certificateARNs))
		for certARN := range spec.certificateARNs {
			certificateARNs = append(certificateARNs, certARN)
		}
		sort.Slice(certificateARNs, func(i, j int) bool {
			return certificateARNs[i] < certificateARNs[j]
		})

		// Add an HTTPS Listener resource with the first certificate as the default one
		if spec.loadbalancerType == LoadBalancerTypeApplication {
			template.AddResource("HTTPSListener", &cloudformation.ElasticLoadBalancingV2Listener{
				DefaultActions: &cloudformation.ElasticLoadBalancingV2ListenerActionList{
					{
						Type:           cloudformation.String("forward"),
						TargetGroupArn: cloudformation.Ref(httpsTargetGroupName).String(),
					},
				},
				Certificates: &cloudformation.ElasticLoadBalancingV2ListenerCertificatePropertyList{
					{
						CertificateArn: cloudformation.String(certificateARNs[0]),
					},
				},
				LoadBalancerArn: cloudformation.Ref(loadBalancerResourceLogicalID).String(),
				Port:            cloudformation.Integer(443),
				Protocol:        cloudformation.String("HTTPS"),
				SslPolicy:       cloudformation.Ref(parameterListenerSslPolicyParameter).String(),
			})
			if spec.denyInternalDomains {
				template.AddResource(
					"HTTPSRuleBlockInternalTraffic",
					generateDenyInternalTrafficRule(
						"HTTPSListener",
						internalTrafficDenyRulePriority,
						spec.internalDomains,
						spec.denyInternalDomainsResponse,
					),
				)
			}
		} else if spec.loadbalancerType == LoadBalancerTypeNetwork {
			template.AddResource("HTTPSListener", &cloudformation.ElasticLoadBalancingV2Listener{
				DefaultActions: &cloudformation.ElasticLoadBalancingV2ListenerActionList{
					{
						Type:           cloudformation.String("forward"),
						TargetGroupArn: cloudformation.Ref(httpsTargetGroupName).String(),
					},
				},
				Certificates: &cloudformation.ElasticLoadBalancingV2ListenerCertificatePropertyList{
					{
						CertificateArn: cloudformation.String(certificateARNs[0]),
					},
				},
				LoadBalancerArn: cloudformation.Ref(loadBalancerResourceLogicalID).String(),
				Port:            cloudformation.Integer(443),
				Protocol:        cloudformation.String("TLS"),
				SslPolicy:       cloudformation.Ref(parameterListenerSslPolicyParameter).String(),
			})
		}

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
	lbAttrList := make(cloudformation.ElasticLoadBalancingV2LoadBalancerLoadBalancerAttributeList, 0, 5)

	if spec.loadbalancerType == LoadBalancerTypeApplication {
		lbAttrList = append(lbAttrList,
			cloudformation.ElasticLoadBalancingV2LoadBalancerLoadBalancerAttribute{
				Key:   cloudformation.String("idle_timeout.timeout_seconds"),
				Value: cloudformation.String(fmt.Sprintf("%d", spec.idleConnectionTimeoutSeconds)),
			},
		)

		lbAttrList = append(lbAttrList,
			cloudformation.ElasticLoadBalancingV2LoadBalancerLoadBalancerAttribute{
				Key:   cloudformation.String("routing.http2.enabled"),
				Value: cloudformation.String(fmt.Sprintf("%t", spec.http2)),
			},
		)
	}

	if spec.loadbalancerType == LoadBalancerTypeNetwork {
		lbAttrList = append(lbAttrList,
			cloudformation.ElasticLoadBalancingV2LoadBalancerLoadBalancerAttribute{
				Key:   cloudformation.String("load_balancing.cross_zone.enabled"),
				Value: cloudformation.String(fmt.Sprintf("%t", spec.nlbCrossZone)),
			},
		)

		lbAttrList = append(lbAttrList,
			cloudformation.ElasticLoadBalancingV2LoadBalancerLoadBalancerAttribute{
				// https://docs.aws.amazon.com/elasticloadbalancing/latest/network/network-load-balancers.html#load-balancer-attributes
				// https://docs.aws.amazon.com/elasticloadbalancing/latest/network/network-load-balancers.html#zonal-dns-affinity
				Key: cloudformation.String("dns_record.client_routing_policy"),
				// availability_zone_affinity 100%
				// partial_availability_zone_affinity 85%
				// any_availability_zone 0%
				Value: cloudformation.String(spec.nlbZoneAffinity),
			},
		)
	}

	if spec.albLogsS3Bucket != "" {
		lbAttrList = append(lbAttrList,
			cloudformation.ElasticLoadBalancingV2LoadBalancerLoadBalancerAttribute{
				Key:   cloudformation.String("access_logs.s3.enabled"),
				Value: cloudformation.String("true"),
			},
		)
		lbAttrList = append(lbAttrList,
			cloudformation.ElasticLoadBalancingV2LoadBalancerLoadBalancerAttribute{
				Key:   cloudformation.String("access_logs.s3.bucket"),
				Value: cloudformation.String(spec.albLogsS3Bucket),
			},
		)
		if spec.albLogsS3Prefix != "" {
			lbAttrList = append(lbAttrList,
				cloudformation.ElasticLoadBalancingV2LoadBalancerLoadBalancerAttribute{
					Key:   cloudformation.String("access_logs.s3.prefix"),
					Value: cloudformation.String(spec.albLogsS3Prefix),
				},
			)
		}
	} else {
		lbAttrList = append(lbAttrList,
			cloudformation.ElasticLoadBalancingV2LoadBalancerLoadBalancerAttribute{
				Key:   cloudformation.String("access_logs.s3.enabled"),
				Value: cloudformation.String("false"),
			},
		)
	}

	lb := &cloudformation.ElasticLoadBalancingV2LoadBalancer{
		LoadBalancerAttributes: &lbAttrList,

		IPAddressType: cloudformation.Ref(parameterIpAddressTypeParameter).String(),
		Scheme:        cloudformation.Ref(parameterLoadBalancerSchemeParameter).String(),
		Subnets:       cloudformation.Ref(parameterLoadBalancerSubnetsParameter).StringList(),
		Tags: &cloudformation.TagList{
			{
				Key:   cloudformation.String("StackName"),
				Value: cloudformation.Ref("AWS::StackName").String(),
			},
		},
	}

	// Security groups can't be set for 'network' load balancers
	if spec.loadbalancerType != LoadBalancerTypeNetwork {
		lb.SecurityGroups = cloudformation.Ref(parameterLoadBalancerSecurityGroupParameter).StringList()
	}

	// TODO(mlarsen): hack to only set type on "new" stacks where this
	// features was enabled. Adding the Type value for existing Load
	// Balancers will cause them to be recreated which is disruptive (and
	// breaks because AWS tries to attach the same TG to multiple LBs).
	// Can be removed in a later version.
	if spec.loadbalancerType == LoadBalancerTypeApplication || spec.loadbalancerType == LoadBalancerTypeNetwork {
		lb.Type = cloudformation.Ref(parameterLoadBalancerTypeParameter).String()
	}

	template.AddResource(loadBalancerResourceLogicalID, lb)

	if spec.loadbalancerType == LoadBalancerTypeApplication && spec.wafWebAclId != "" {
		if strings.HasPrefix(spec.wafWebAclId, "arn:aws:wafv2:") {
			template.AddResource("WAFAssociation", &cloudformation.WAFv2WebACLAssociation{
				ResourceArn: cloudformation.Ref(loadBalancerResourceLogicalID).String(),
				WebACLArn:   cloudformation.Ref(parameterLoadBalancerWAFWebACLIDParameter).String(),
			})
		} else {
			template.AddResource("WAFAssociation", &cloudformation.WAFRegionalWebACLAssociation{
				ResourceArn: cloudformation.Ref(loadBalancerResourceLogicalID).String(),
				WebACLID:    cloudformation.Ref(parameterLoadBalancerWAFWebACLIDParameter).String(),
			})
		}
	}

	for idx, alarm := range spec.cwAlarms {
		resourceName := fmt.Sprintf("CloudWatchAlarm%d", idx)
		template.AddResource(resourceName, &cloudformation.CloudWatchAlarm{
			ActionsEnabled:                   alarm.ActionsEnabled,
			AlarmActions:                     alarm.AlarmActions,
			AlarmDescription:                 alarm.AlarmDescription,
			AlarmName:                        normalizeCloudWatchAlarmName(alarm.AlarmName),
			ComparisonOperator:               alarm.ComparisonOperator,
			Dimensions:                       normalizeCloudWatchAlarmDimensions(alarm.Dimensions),
			EvaluateLowSampleCountPercentile: alarm.EvaluateLowSampleCountPercentile,
			EvaluationPeriods:                alarm.EvaluationPeriods,
			ExtendedStatistic:                alarm.ExtendedStatistic,
			InsufficientDataActions:          alarm.InsufficientDataActions,
			MetricName:                       alarm.MetricName,
			Namespace:                        normalizeCloudWatchAlarmNamespace(alarm.Namespace),
			OKActions:                        alarm.OKActions,
			Period:                           alarm.Period,
			Statistic:                        alarm.Statistic,
			Threshold:                        alarm.Threshold,
			TreatMissingData:                 alarm.TreatMissingData,
			Unit:                             alarm.Unit,
		})
	}

	stackTemplate, err := json.MarshalIndent(template, "", "    ")
	if err != nil {
		return "", err
	}

	return string(stackTemplate), nil
}

func generateDenyInternalTrafficRule(listenerName string, rulePriority int64, internalDomains []string, resp denyResp) cloudformation.ElasticLoadBalancingV2ListenerRule {
	values := cloudformation.StringList()
	for _, domain := range internalDomains {
		values.Literal = append(values.Literal, cloudformation.String(domain))
	}

	return cloudformation.ElasticLoadBalancingV2ListenerRule{
		Conditions: &cloudformation.ElasticLoadBalancingV2ListenerRuleRuleConditionList{
			cloudformation.ElasticLoadBalancingV2ListenerRuleRuleCondition{
				Field:  cloudformation.String(listenerRuleConditionHostField),
				Values: values,
			},
		},
		Actions: &cloudformation.ElasticLoadBalancingV2ListenerRuleActionList{
			cloudformation.ElasticLoadBalancingV2ListenerRuleAction{
				Type: cloudformation.String(listenerRuleActionTypeFixedRes),
				FixedResponseConfig: &cloudformation.ElasticLoadBalancingV2ListenerRuleFixedResponseConfig{
					ContentType: cloudformation.String(resp.contentType),
					MessageBody: cloudformation.String(resp.body),
					StatusCode:  cloudformation.String(fmt.Sprintf("%d", resp.statusCode)),
				},
			},
		},
		Priority:    cloudformation.Integer(rulePriority),
		ListenerArn: cloudformation.Ref(listenerName).String(),
	}
}

func newTargetGroup(spec *stackSpec, targetPortParameter string) *cloudformation.ElasticLoadBalancingV2TargetGroup {
	var targetType *cloudformation.StringExpr
	if spec.targetType != "" {
		targetType = cloudformation.String(string(spec.targetType))
	}

	protocol := "HTTP"
	healthCheckProtocol := "HTTP"
	healthyThresholdCount, unhealthyThresholdCount := spec.albHealthyThresholdCount, spec.albUnhealthyThresholdCount
	if spec.loadbalancerType == LoadBalancerTypeNetwork {
		protocol = "TCP"
		healthCheckProtocol = "HTTP"
		// For NLBs the healthy and unhealthy threshold count value must be equal
		healthyThresholdCount, unhealthyThresholdCount = spec.nlbHealthyThresholdCount, spec.nlbHealthyThresholdCount
	} else if spec.targetHTTPS {
		protocol = "HTTPS"
		healthCheckProtocol = "HTTPS"
	}

	targetGroup := &cloudformation.ElasticLoadBalancingV2TargetGroup{
		TargetGroupAttributes: &cloudformation.ElasticLoadBalancingV2TargetGroupTargetGroupAttributeList{
			{
				Key:   cloudformation.String("deregistration_delay.timeout_seconds"),
				Value: cloudformation.String(fmt.Sprintf("%d", spec.deregistrationDelayTimeoutSeconds)),
			},
		},
		HealthCheckIntervalSeconds: cloudformation.Ref(parameterTargetGroupHealthCheckIntervalParameter).Integer(),
		HealthCheckPath:            cloudformation.Ref(parameterTargetGroupHealthCheckPathParameter).String(),
		HealthCheckPort:            cloudformation.Ref(parameterTargetGroupHealthCheckPortParameter).String(),
		HealthCheckProtocol:        cloudformation.String(healthCheckProtocol),
		HealthyThresholdCount:      cloudformation.Integer(int64(healthyThresholdCount)),
		UnhealthyThresholdCount:    cloudformation.Integer(int64(unhealthyThresholdCount)),
		Port:                       cloudformation.Ref(targetPortParameter).Integer(),
		Protocol:                   cloudformation.String(protocol),
		TargetType:                 targetType,
		VPCID:                      cloudformation.Ref(parameterTargetGroupVPCIDParameter).String(),
	}

	// custom target group healthcheck only supported when the target group protocol is != TCP
	if protocol != "TCP" {
		targetGroup.HealthCheckTimeoutSeconds = cloudformation.Ref(parameterTargetGroupHealthCheckTimeoutParameter).Integer()
	}
	return targetGroup
}
