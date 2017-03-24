package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/aws/aws-sdk-go/service/elbv2/elbv2iface"
)

type elbv2APIOutputs struct {
	createLoadBalancer        *apiResponse
	createTargetGroup         *apiResponse
	createListener            *apiResponse
	deleteLoadBalancer        *apiResponse
	deleteTargetGroup         *apiResponse
	deleteListener            *apiResponse
	describeLoadBalancers     *apiResponse
	describeTags              *apiResponse
	describeListeners         *apiResponse
	describeAutoScalingGroups *apiResponse
}

type mockELBV2Client struct {
	elbv2iface.ELBV2API
	outputs elbv2APIOutputs
}

func (m *mockELBV2Client) CreateLoadBalancer(*elbv2.CreateLoadBalancerInput) (*elbv2.CreateLoadBalancerOutput, error) {
	if out, ok := m.outputs.createLoadBalancer.response.(*elbv2.CreateLoadBalancerOutput); ok {
		return out, m.outputs.createLoadBalancer.err
	}
	return nil, m.outputs.createLoadBalancer.err
}

func (m *mockELBV2Client) CreateTargetGroup(*elbv2.CreateTargetGroupInput) (*elbv2.CreateTargetGroupOutput, error) {
	if out, ok := m.outputs.createTargetGroup.response.(*elbv2.CreateTargetGroupOutput); ok {
		return out, m.outputs.createTargetGroup.err
	}
	return nil, m.outputs.createTargetGroup.err
}

func (m *mockELBV2Client) CreateListener(*elbv2.CreateListenerInput) (*elbv2.CreateListenerOutput, error) {
	if out, ok := m.outputs.createListener.response.(*elbv2.CreateListenerOutput); ok {
		return out, m.outputs.createListener.err
	}
	return nil, m.outputs.createListener.err
}

func (m *mockELBV2Client) DeleteLoadBalancer(*elbv2.DeleteLoadBalancerInput) (*elbv2.DeleteLoadBalancerOutput, error) {
	if out, ok := m.outputs.deleteLoadBalancer.response.(*elbv2.DeleteLoadBalancerOutput); ok {
		return out, m.outputs.createListener.err
	}
	return nil, m.outputs.deleteLoadBalancer.err

}

func (m *mockELBV2Client) DeleteTargetGroup(*elbv2.DeleteTargetGroupInput) (*elbv2.DeleteTargetGroupOutput, error) {
	if out, ok := m.outputs.deleteTargetGroup.response.(*elbv2.DeleteTargetGroupOutput); ok {
		return out, m.outputs.deleteTargetGroup.err
	}
	return nil, m.outputs.deleteTargetGroup.err
}

func (m *mockELBV2Client) DeleteListener(*elbv2.DeleteListenerInput) (*elbv2.DeleteListenerOutput, error) {
	if out, ok := m.outputs.deleteListener.response.(*elbv2.DeleteListenerOutput); ok {
		return out, m.outputs.deleteListener.err
	}
	return nil, m.outputs.deleteListener.err
}

func (m *mockELBV2Client) DescribeLoadBalancers(*elbv2.DescribeLoadBalancersInput) (*elbv2.DescribeLoadBalancersOutput, error) {
	if out, ok := m.outputs.describeLoadBalancers.response.(*elbv2.DescribeLoadBalancersOutput); ok {
		return out, m.outputs.describeLoadBalancers.err
	}
	return nil, m.outputs.describeLoadBalancers.err
}

func (m *mockELBV2Client) DescribeTags(*elbv2.DescribeTagsInput) (*elbv2.DescribeTagsOutput, error) {
	if out, ok := m.outputs.describeTags.response.(*elbv2.DescribeTagsOutput); ok {
		return out, m.outputs.describeTags.err
	}
	return nil, m.outputs.describeTags.err
}

func (m *mockELBV2Client) DescribeListeners(*elbv2.DescribeListenersInput) (*elbv2.DescribeListenersOutput, error) {
	if out, ok := m.outputs.describeListeners.response.(*elbv2.DescribeListenersOutput); ok {
		return out, m.outputs.describeListeners.err
	}
	return nil, m.outputs.describeListeners.err
}

func mockCLBOutput(dnsName, arn string) *elbv2.CreateLoadBalancerOutput {
	return &elbv2.CreateLoadBalancerOutput{
		LoadBalancers: []*elbv2.LoadBalancer{
			{
				DNSName:         aws.String(dnsName),
				LoadBalancerArn: aws.String(arn),
			},
		},
	}
}

func mockCTGOutput(arn string) *elbv2.CreateTargetGroupOutput {
	return &elbv2.CreateTargetGroupOutput{
		TargetGroups: []*elbv2.TargetGroup{
			{TargetGroupArn: aws.String(arn)},
		},
	}
}

func mockCLOutput(arn string) *elbv2.CreateListenerOutput {
	return &elbv2.CreateListenerOutput{
		Listeners: []*elbv2.Listener{
			{
				ListenerArn: aws.String(arn),
			},
		},
	}
}

type lbMock struct {
	name, arn, dnsName string
}

func mockDLBOutput(mocks ...lbMock) *elbv2.DescribeLoadBalancersOutput {
	lbs := make([]*elbv2.LoadBalancer, len(mocks))
	for i, mock := range mocks {
		lbs[i] = &elbv2.LoadBalancer{
			LoadBalancerName: aws.String(mock.name),
			DNSName:          aws.String(mock.dnsName),
			LoadBalancerArn:  aws.String(mock.arn),
		}
	}
	return &elbv2.DescribeLoadBalancersOutput{LoadBalancers: lbs}
}

func mockDTOutput(resourceTags awsTags) *elbv2.DescribeTagsOutput {
	res := &elbv2.DescribeTagsOutput{
		TagDescriptions: make([]*elbv2.TagDescription, 0, len(resourceTags)),
	}
	for resourceArn, tags := range resourceTags {
		td := &elbv2.TagDescription{
			ResourceArn: aws.String(resourceArn),
			Tags:        make([]*elbv2.Tag, 0, len(tags)),
		}
		for k, v := range tags {
			td.Tags = append(td.Tags, &elbv2.Tag{Key: aws.String(k), Value: aws.String(v)})
		}
		res.TagDescriptions = append(res.TagDescriptions, td)
	}

	return res
}

type listenerMock struct {
	port    int64
	arn     string
	certARN string
}

func mockDLOutput(targetGroupARN string, mocks ...listenerMock) *elbv2.DescribeListenersOutput {
	listeners := make([]*elbv2.Listener, len(mocks))
	for i, mock := range mocks {
		listeners[i] = mockElbv2Listener(targetGroupARN, mock)
	}
	return &elbv2.DescribeListenersOutput{Listeners: listeners}
}

func mockLoadBalancer(name, arn, dnsName string, listeners *loadBalancerListeners) *LoadBalancer {
	return &LoadBalancer{
		name:      name,
		arn:       arn,
		dnsName:   dnsName,
		listeners: listeners,
	}
}

func mockListeners(targetGroupARN string, http *loadBalancerListener, https *loadBalancerListener) *loadBalancerListeners {
	return &loadBalancerListeners{
		http:           http,
		https:          https,
		targetGroupARN: targetGroupARN,
	}
}

func mockListener(port int64, arn, certificateARN string) *loadBalancerListener {
	return &loadBalancerListener{
		port:           port,
		arn:            arn,
		certificateARN: certificateARN,
	}
}

func mockElbv2Listener(targetGroupARN string, mock listenerMock) *elbv2.Listener {
	certs := make([]*elbv2.Certificate, 0)
	var proto string = elbv2.ProtocolEnumHttp
	if mock.certARN != "" {
		certs = append(certs, &elbv2.Certificate{CertificateArn: aws.String(mock.certARN)})
		proto = elbv2.ProtocolEnumHttps
	}
	actions := make([]*elbv2.Action, 0)
	if targetGroupARN != "" {
		actions = append(actions, &elbv2.Action{TargetGroupArn: aws.String(targetGroupARN)})
	}
	return &elbv2.Listener{
		Certificates:   certs,
		Port:           aws.Int64(mock.port),
		ListenerArn:    aws.String(mock.arn),
		Protocol:       aws.String(proto),
		DefaultActions: actions,
	}
}
