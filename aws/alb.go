package aws

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/aws/aws-sdk-go/service/elbv2/elbv2iface"
	"github.com/pkg/errors"
	"log"
	"strings"
)

// LoadBalancer is a simple wrapper around an AWS Load Balancer details
type LoadBalancer struct {
	name     string
	arn      string
	dnsName  string
	listener *loadBalancerListener
}

// Name returns the load balancer friendly name
func (lb *LoadBalancer) Name() string {
	return lb.name
}

// ARN returns the load balancer ARN
func (lb *LoadBalancer) ARN() string {
	return lb.arn
}

// DNSName returns the FQDN for the load balancer. It's usually prefixed by its Name
func (lb *LoadBalancer) DNSName() string {
	return lb.dnsName
}

type loadBalancerListener struct {
	port           int64
	arn            string
	certificateARN string
}

const kubernetesCreatorTag = "kubernetes:application"

func findLoadBalancersWithCertificateID(elbv2 elbv2iface.ELBV2API, certificateARN string) (*LoadBalancer, error) {
	// TODO: paged results
	resp, err := elbv2.DescribeLoadBalancers(nil)
	if err != nil {
		return nil, err
	}

	// TODO: filter for ALBs with a given set of tags? For ex.: KubernetesCluster=foo
	for _, lb := range resp.LoadBalancers {
		listeners, err := getListeners(elbv2, aws.StringValue(lb.LoadBalancerArn))
		if err != nil {
			log.Printf("failed to describe listeners for load balancer %q: %v\n", lb.LoadBalancerName, err)
			continue
		}
		for _, listener := range listeners {
			for _, cert := range listener.Certificates {
				certARN := aws.StringValue(cert.CertificateArn)
				if certARN == certificateARN {
					return &LoadBalancer{
						name: aws.StringValue(lb.LoadBalancerName),
						arn:  aws.StringValue(lb.LoadBalancerArn),
						listener: &loadBalancerListener{
							port:           aws.Int64Value(listener.Port),
							arn:            aws.StringValue(listener.ListenerArn),
							certificateARN: certARN,
						},
					}, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("load balancer with certificate %q not found", certificateARN)
}

func getListeners(alb elbv2iface.ELBV2API, loadBalancerARN string) ([]*elbv2.Listener, error) {
	// TODO: paged results
	params := &elbv2.DescribeListenersInput{
		LoadBalancerArn: aws.String(loadBalancerARN),
	}
	resp, err := alb.DescribeListeners(params)

	if err != nil {
		return nil, err
	}
	return resp.Listeners, nil
}

type createLoadBalancerSpec struct {
	name            string
	scheme          string
	subnets         []string
	certificateARN  string
	securityGroupID string
	clusterName     string
	targetGroupARNs []string
}

func createLoadBalancer(alb elbv2iface.ELBV2API, spec *createLoadBalancerSpec) (*LoadBalancer, error) {
	var name = normalizeLoadBalancerName(spec.certificateARN)
	params := &elbv2.CreateLoadBalancerInput{
		Name:    aws.String(name),
		Subnets: aws.StringSlice(spec.subnets),
		Scheme:  aws.String(spec.scheme),
		SecurityGroups: []*string{
			aws.String(spec.securityGroupID),
		},
		Tags: []*elbv2.Tag{
			{
				Key:   aws.String(kubernetesClusterTag),
				Value: aws.String(spec.clusterName),
			},
			{
				Key:   aws.String(kubernetesCreatorTag),
				Value: aws.String("kube-ingress-aws-controller"),
			},
		},
	}
	resp, err := alb.CreateLoadBalancer(params)

	if err != nil {
		return nil, err
	}

	if len(resp.LoadBalancers) < 1 {
		return nil, errors.New("request to create ALB succeeded but returned no items")
	}

	newLoadBalancer := resp.LoadBalancers[0]
	loadBalancerARN := aws.StringValue(newLoadBalancer.LoadBalancerArn)
	newListener, err := createListener(alb, loadBalancerARN, spec)
	if err != nil {
		// TODO: delete just created LB?
		return nil, err
	}

	return &LoadBalancer{
		arn:      loadBalancerARN,
		name:     name,
		dnsName:  aws.StringValue(newLoadBalancer.DNSName),
		listener: newListener,
	}, nil
}

func normalizeLoadBalancerName(name string) string {
	fields := strings.Split(name, "/")
	if len(fields) >= 2 {
		name = strings.Replace(fields[1], "-", "", -1)
	}
	if len(name) > 32 {
		name = name[:32]
	}
	return name
}

func createListener(alb elbv2iface.ELBV2API, loadBalancerARN string, spec *createLoadBalancerSpec) (*loadBalancerListener, error) {
	actions := make([]*elbv2.Action, len(spec.targetGroupARNs))
	for i, tg := range spec.targetGroupARNs {
		actions[i] = &elbv2.Action{TargetGroupArn: aws.String(tg), Type: aws.String(elbv2.ActionTypeEnumForward)}
	}
	params := &elbv2.CreateListenerInput{
		Certificates: []*elbv2.Certificate{
			{
				CertificateArn: aws.String(spec.certificateARN),
			},
		},
		LoadBalancerArn: aws.String(loadBalancerARN),
		Port:            aws.Int64(443),
		Protocol:        aws.String(elbv2.ProtocolEnumHttps),
		DefaultActions:  actions,
	}

	resp, err := alb.CreateListener(params)
	if err != nil {
		return nil, err
	}
	if len(resp.Listeners) < 1 {
		return nil, errors.New("request to create Listener succeeded but returned no items")
	}
	l := resp.Listeners[0]
	return &loadBalancerListener{
		arn:            aws.StringValue(l.ListenerArn),
		port:           aws.Int64Value(l.Port),
		certificateARN: spec.certificateARN,
	}, nil
}

func createDefaultTargetGroup(alb elbv2iface.ELBV2API, clusterName string, vpcID string) ([]string, error) {
	params := &elbv2.CreateTargetGroupInput{
		HealthCheckPath: aws.String("/healthz"),
		Port:            aws.Int64(9990),
		Protocol:        aws.String(elbv2.ProtocolEnumHttp),
		VpcId:           aws.String(vpcID),
		Name:            aws.String(fmt.Sprintf("%s-worker-tg", clusterName)),
	}
	resp, err := alb.CreateTargetGroup(params)
	if err != nil {
		return nil, err
	}

	if len(resp.TargetGroups) < 1 {
		return nil, errors.New("request to create default Target Group succeeded but returned no items")
	}

	ret := make([]string, len(resp.TargetGroups))
	for i, tg := range resp.TargetGroups {
		ret[i] = aws.StringValue(tg.TargetGroupArn)
	}

	return ret, nil
}
