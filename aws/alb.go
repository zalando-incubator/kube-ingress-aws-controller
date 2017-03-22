package aws

import (
	"crypto/sha1"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"encoding/hex"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/aws/aws-sdk-go/service/elbv2/elbv2iface"
	"github.com/pkg/errors"
)

// LoadBalancer is a simple wrapper around an AWS Load Balancer details.
type LoadBalancer struct {
	name           string
	arn            string
	dnsName        string
	listeners      *loadBalancerListeners
	scheduleDelete *time.Time
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

func (lb *LoadBalancer) IsDeleteDeleteScheduled() bool {
	return lb.scheduleDelete != nil && *lb.scheduleDelete != time.Time{}
}

// ScheduleDelete sets the time to schedule to now + given duration
func (lb *LoadBalancer) ScheduleDelete(d *time.Duration) {
	lb.scheduleDelete = time.Now().Add(d)
}

// GetScheduleDelete returns the time after when you are allowed to
// delete this LoadBalancer
func (lb *LoadBalancer) GetScheduleDelete() *time.Time {
	return lb.scheduleDelete
}

func (lb *LoadBalancer) CertificateARN() string {
	if lb.listeners == nil || lb.listeners.https == nil {
		return ""
	}
	return lb.listeners.https.certificateARN
}

type loadBalancerListeners struct {
	http           *loadBalancerListener
	https          *loadBalancerListener
	targetGroupARN string
}

type loadBalancerListener struct {
	port           int64
	arn            string
	certificateARN string
}

const (
	kubernetesCreatorTag   = "kubernetes:application"
	kubernetesCreatorValue = "kube-ingress-aws-controller"
	httpListenerPort       = 80
	httpsListenerPort      = 443
)

func findManagedLBWithCertificateID(elbv2 elbv2iface.ELBV2API, clusterID string, certificateARN string) (*LoadBalancer, error) {
	lbs, err := findManagedLoadBalancers(elbv2, clusterID)
	if err != nil {
		return nil, err
	}
	for _, lb := range lbs {
		if lb.CertificateARN() == certificateARN {
			return lb, nil
		}
	}
	return nil, nil
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
	clusterID       string
	vpcID           string
	healthCheck     healthCheck
}

type healthCheck struct {
	path string
	port uint16
}

func createLoadBalancer(svc elbv2iface.ELBV2API, spec *createLoadBalancerSpec) (*LoadBalancer, error) {
	var name = normalizeLoadBalancerName(spec.clusterID, spec.certificateARN)
	params := &elbv2.CreateLoadBalancerInput{
		Name:    aws.String(name),
		Subnets: aws.StringSlice(spec.subnets),
		Scheme:  aws.String(spec.scheme),
		SecurityGroups: []*string{
			aws.String(spec.securityGroupID),
		},
		Tags: []*elbv2.Tag{
			{
				Key:   aws.String(nameTag),
				Value: aws.String(name),
			},
			{
				Key:   aws.String(clusterIDTag),
				Value: aws.String(spec.clusterID),
			},
			{
				Key:   aws.String(kubernetesCreatorTag),
				Value: aws.String(kubernetesCreatorValue),
			},
		},
	}
	resp, err := svc.CreateLoadBalancer(params)

	if err != nil {
		return nil, err
	}

	if len(resp.LoadBalancers) < 1 {
		return nil, errors.New("request to create ALB succeeded but returned no items")
	}

	newLoadBalancer := resp.LoadBalancers[0]
	loadBalancerARN := aws.StringValue(newLoadBalancer.LoadBalancerArn)
	targetGroupARN, err := createDefaultTargetGroup(svc, name, spec.vpcID, spec.healthCheck)
	newHTTPSListener, err := createListener(svc, loadBalancerARN, targetGroupARN, httpsListenerPort, elbv2.ProtocolEnumHttps, spec.certificateARN)
	if err != nil {
		// TODO: delete just created LB?
		return nil, err
	}
	newHTTPListener, err := createListener(svc, loadBalancerARN, targetGroupARN, httpListenerPort, elbv2.ProtocolEnumHttp, "")
	if err != nil {
		// TODO: delete just created LB?
		return nil, err
	}

	return &LoadBalancer{
		arn:     loadBalancerARN,
		name:    name,
		dnsName: aws.StringValue(newLoadBalancer.DNSName),
		listeners: &loadBalancerListeners{
			https:          newHTTPSListener,
			http:           newHTTPListener,
			targetGroupARN: targetGroupARN,
		},
	}, nil
}

// Hash ARN, keep last 7 hex chars
// Prepend 24 chars from normalized ClusterID with a '-' in between
// Normalization of ClusterID should replace all non valid chars
// Valid sets: a-z,A-Z,0-9,-
// Replacement char: -
// Squeeze and strip from beginning and/or end
var normalizationRegex = regexp.MustCompile("[^A-Za-z0-9-]+")

const (
	shortHashLen    = 7
	maxClusterIDLen = 24
)

func normalizeLoadBalancerName(clusterID string, certificateARN string) string {
	hasher := sha1.New()
	hasher.Write([]byte(certificateARN))
	hash := hex.EncodeToString(hasher.Sum(nil))
	hashLen := len(hash)
	if hashLen > shortHashLen {
		hash = strings.ToLower(hash[hashLen-shortHashLen:])
	}

	normalizedClusterID := normalizationRegex.ReplaceAllString(clusterID, "-")
	lenClusterID := len(normalizedClusterID)
	if lenClusterID > maxClusterIDLen {
		normalizedClusterID = strings.TrimPrefix(normalizedClusterID[lenClusterID-maxClusterIDLen:], "-")
	}

	return fmt.Sprintf("%s-%s", normalizedClusterID, hash)
}

func createDefaultTargetGroup(alb elbv2iface.ELBV2API, name string, vpcID string, hc healthCheck) (string, error) {
	params := &elbv2.CreateTargetGroupInput{
		HealthCheckPath: aws.String(hc.path),
		Port:            aws.Int64(int64(hc.port)),
		Protocol:        aws.String(elbv2.ProtocolEnumHttp),
		VpcId:           aws.String(vpcID),
		Name:            aws.String(name),
	}
	resp, err := alb.CreateTargetGroup(params)
	if err != nil {
		return "", err
	}

	if len(resp.TargetGroups) < 1 {
		return "", errors.New("request to create default Target Group succeeded but returned no items")
	}

	return aws.StringValue(resp.TargetGroups[0].TargetGroupArn), nil
}

func createListener(alb elbv2iface.ELBV2API, loadBalancerARN string, targetGroupARN string, port int64, protocol string, certificateARN string) (*loadBalancerListener, error) {
	var certificates []*elbv2.Certificate

	if protocol == elbv2.ProtocolEnumHttps {
		certificates = []*elbv2.Certificate{
			{
				CertificateArn: aws.String(certificateARN),
			},
		}
	}

	params := &elbv2.CreateListenerInput{
		Certificates:    certificates,
		LoadBalancerArn: aws.String(loadBalancerARN),
		Port:            aws.Int64(port),
		Protocol:        aws.String(protocol),
		DefaultActions: []*elbv2.Action{
			{
				TargetGroupArn: aws.String(targetGroupARN),
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
		certificateARN: certificateARN,
	}, nil
}

func findManagedLoadBalancers(svc elbv2iface.ELBV2API, clusterID string) ([]*LoadBalancer, error) {
	resp, err := svc.DescribeLoadBalancers(nil)
	if err != nil {
		return nil, err
	}

	if len(resp.LoadBalancers) == 0 {
		return nil, ErrLoadBalancerNotFound
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

	for _, lb := range resp.LoadBalancers {
		for _, td := range r.TagDescriptions {
			tags := convertElbv2Tags(td.Tags)
			if isManagedLoadBalancer(tags, clusterID) {
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
				if len(listener.DefaultActions) < 1 {
					log.Printf("load balancer %q doesn't have the default target group", loadBalancerARN)
					continue
				}
				if aws.StringValue(lb.LoadBalancerArn) == aws.StringValue(td.ResourceArn) {
					lb := &LoadBalancer{
						name:    aws.StringValue(lb.LoadBalancerName),
						dnsName: aws.StringValue(lb.DNSName),
						arn:     aws.StringValue(td.ResourceArn),
						listeners: &loadBalancerListeners{
							https: &loadBalancerListener{
								port:           aws.Int64Value(listener.Port),
								arn:            aws.StringValue(listener.ListenerArn),
								certificateARN: certARN,
							},
							targetGroupARN: aws.StringValue(listener.DefaultActions[0].TargetGroupArn),
						},
					}

					httpListener := findHTTPListener(listeners)
					if httpListener != nil {
						lb.listeners.http = &loadBalancerListener{
							port: aws.Int64Value(httpListener.Port),
							arn:  aws.StringValue(httpListener.ListenerArn),
						}
					}

					loadBalancers = append(loadBalancers, lb)
				}
			}
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

// finds HTTP listener from a list of listeners based on port and protocol.
func findHTTPListener(listeners []*elbv2.Listener) *elbv2.Listener {
	for _, l := range listeners {
		if aws.Int64Value(l.Port) == httpListenerPort &&
			aws.StringValue(l.Protocol) == elbv2.ProtocolEnumHttp {
			return l
		}
	}
	return nil
}

func isManagedLoadBalancer(tags map[string]string, clusterID string) bool {
	if tags[kubernetesCreatorTag] != kubernetesCreatorValue {
		return false
	}
	if tags[clusterIDTag] != clusterID {
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

func deleteTargetGroup(svc elbv2iface.ELBV2API, targetGroupARN string) error {
	params := &elbv2.DeleteTargetGroupInput{TargetGroupArn: aws.String(targetGroupARN)}
	_, err := svc.DeleteTargetGroup(params)
	if err != nil {
		return err
	}
	return nil
}

func deleteListener(svc elbv2iface.ELBV2API, listenerARN string) error {
	params := &elbv2.DeleteListenerInput{ListenerArn: aws.String(listenerARN)}
	_, err := svc.DeleteListener(params)
	if err != nil {
		return err
	}
	return nil
}
