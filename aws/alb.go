package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/aws/aws-sdk-go/service/elbv2/elbv2iface"
	"github.com/pkg/errors"
	"log"
	"strings"
)

// LoadBalancer is a simple wrapper around an AWS Load Balancer details.
type LoadBalancer struct {
	name     string
	arn      string
	dnsName  string
	listener *loadBalancerListener
}

// Name returns the load balancer friendly name.
func (lb *LoadBalancer) Name() string {
	return lb.name
}

// ARN returns the load balancer ARN.
func (lb *LoadBalancer) ARN() string {
	return lb.arn
}

// DNSName returns the FQDN for the load balancer. It's usually prefixed by its Name.
func (lb *LoadBalancer) DNSName() string {
	return lb.dnsName
}

func (lb *LoadBalancer) CertificateARN() string {
	if lb.listener == nil {
		return ""
	}
	return lb.listener.certificateARN
}

type loadBalancerListener struct {
	port           int64
	arn            string
	certificateARN string
}

const (
	kubernetesCreatorTag   = "kubernetes:application"
	kubernetesCreatorValue = "kube-ingress-aws-controller"
	maxResourceNameLen     = 32
)

func findLoadBalancerWithCertificateID(elbv2 elbv2iface.ELBV2API, certificateARN string) (*LoadBalancer, error) {
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

	return nil, ErrLoadBalancerNotFound
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
	stackName       string
	targetGroupARN  string
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
				Key:   aws.String("Name"),
				Value: aws.String(spec.stackName),
			},
			{
				Key:   aws.String(kubernetesCreatorTag),
				Value: aws.String(kubernetesCreatorValue),
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
	if len(name) > maxResourceNameLen {
		name = name[:maxResourceNameLen]
	}
	return name
}

func createListener(alb elbv2iface.ELBV2API, loadBalancerARN string, spec *createLoadBalancerSpec) (*loadBalancerListener, error) {
	params := &elbv2.CreateListenerInput{
		Certificates: []*elbv2.Certificate{
			{
				CertificateArn: aws.String(spec.certificateARN),
			},
		},
		LoadBalancerArn: aws.String(loadBalancerARN),
		Port:            aws.Int64(443),
		Protocol:        aws.String(elbv2.ProtocolEnumHttps),
		DefaultActions: []*elbv2.Action{
			{
				TargetGroupArn: aws.String(spec.targetGroupARN),
				Type:           aws.String(elbv2.ActionTypeEnumForward),
			},
		},
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

func findManagedLoadBalancers(svc elbv2iface.ELBV2API, clusterName string) ([]*LoadBalancer, error) {
	resp, err := svc.DescribeLoadBalancers(nil)
	if err != nil {
		return nil, err
	}

	loadBalancerARNs := make([]*string, len(resp.LoadBalancers))
	for i, lb := range resp.LoadBalancers {
		loadBalancerARNs[i] = lb.LoadBalancerArn
	}

	params := &elbv2.DescribeTagsInput{ResourceArns: loadBalancerARNs}
	r, err := svc.DescribeTags(params)
	if err != nil {
		return nil, err
	}

	var loadBalancers []*LoadBalancer
	for _, td := range r.TagDescriptions {
		tags := convertElbv2Tags(td.Tags)
		if isManagedLoadBalancer(tags, clusterName) {
			loadBalancerARN := aws.StringValue(td.ResourceArn)
			listeners, err := getListeners(svc, loadBalancerARN)
			if err != nil {
				log.Printf("failed to describe listeners for load balancer ARN %q: %v\n", loadBalancerARN, err)
				continue
			}

			listener, certARN := findFirstListenerWithAnyCertificate(listeners)
			if len(listeners) < 1 {
				log.Printf("load balancer ARN %q has no certificates\n", loadBalancerARN)
				continue
			}

			loadBalancers = append(loadBalancers, &LoadBalancer{
				arn: aws.StringValue(td.ResourceArn),
				listener: &loadBalancerListener{
					port:           aws.Int64Value(listener.Port),
					arn:            aws.StringValue(listener.ListenerArn),
					certificateARN: certARN,
				},
			})
		}
	}
	return loadBalancers, err
}

func findFirstListenerWithAnyCertificate(listeners []*elbv2.Listener) (*elbv2.Listener, string) {
	for _, l := range listeners {
		for _, c := range l.Certificates {
			if aws.StringValue(c.CertificateArn) != "" {
				return l, aws.StringValue(c.CertificateArn)
			}
		}
	}
	return nil, ""
}

func isManagedLoadBalancer(tags map[string]string, stackName string) bool {
	if tags[kubernetesCreatorTag] != kubernetesCreatorValue {
		return false
	}
	if tags["Name"] != stackName {
		return false
	}
	return true
}

func convertElbv2Tags(tags []*elbv2.Tag) map[string]string {
	ret := make(map[string]string)
	for _, tag := range tags {
		ret[aws.StringValue(tag.Key)] = aws.StringValue(tag.Value)
	}
	return ret
}

func deleteLoadBalancer(svc elbv2iface.ELBV2API, loadBalancerARN string) error {
	params := &elbv2.DeleteLoadBalancerInput{LoadBalancerArn: aws.String(loadBalancerARN)}
	_, err := svc.DeleteLoadBalancer(params)
	if err != nil {
		return err
	}
	return nil
}

func findTargetGroupWithNameTag(svc elbv2iface.ELBV2API, nameTag string) (string, error) {
	resp, err := svc.DescribeTargetGroups(nil)
	if err != nil {
		return "", err
	}
	targetGroupARNs := make([]*string, len(resp.TargetGroups))
	for i, tg := range resp.TargetGroups {
		targetGroupARNs[i] = tg.TargetGroupArn
	}
	params := &elbv2.DescribeTagsInput{ResourceArns: targetGroupARNs}
	r, err := svc.DescribeTags(params)
	if err != nil {
		return "", err
	}

	for _, td := range r.TagDescriptions {
		tags := convertElbv2Tags(td.Tags)
		name, err := getNameTag(tags)
		if err != nil {
			log.Printf("target group %q couldn't be identified:%v\n", aws.StringValue(td.ResourceArn), err)
			continue
		}
		if name == nameTag {
			return aws.StringValue(td.ResourceArn), nil
		}
	}
	return "", ErrTargetGroupNotFound
}
