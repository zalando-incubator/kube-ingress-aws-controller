package main

import (
	"context"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	"github.com/zalando-incubator/kube-ingress-aws-controller/aws"
	"github.com/zalando-incubator/kube-ingress-aws-controller/certs"
	"github.com/zalando-incubator/kube-ingress-aws-controller/kubernetes"

	certsfake "github.com/zalando-incubator/kube-ingress-aws-controller/certs/fake"
	awsmock "github.com/zalando-incubator/kube-ingress-aws-controller/internal/aws/mock"
	kubemock "github.com/zalando-incubator/kube-ingress-aws-controller/internal/kubernetes/mock"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type MinLoadBalancerAgeTestSuite struct {
	suite.Suite

	worker      *worker
	clientELBv2 *awsmock.ELBV2API
	kubeAPI     *kubemock.API
}

func TestMinLoadBalancerAge(t *testing.T) {
	suite.Run(t, new(MinLoadBalancerAgeTestSuite))
}

func (suite *MinLoadBalancerAgeTestSuite) SetupTest() {
	t := suite.T()

	const minLoadBalancerAge = 6 * time.Minute

	// AWS mocks setup
	clientASG := &awsmock.AutoScalingAPI{}
	clientEC2 := &awsmock.EC2API{}
	clientELBv2 := &awsmock.ELBV2API{}
	clientCF := &awsmock.CloudFormationAPI{}

	ca, err := certsfake.NewCA()
	require.NoError(t, err)

	certSummary, err := ca.NewCertificateSummary("arn:test-cert-1", "ingress-1.test")
	require.NoError(t, err)

	certsProvider := &certsfake.CertificateProvider{
		Summaries: []*certs.CertificateSummary{certSummary},
	}

	// Mock setup for EC2
	clientEC2.On("DescribeSecurityGroups", mock.Anything, mock.Anything, mock.Anything).
		Return(&ec2.DescribeSecurityGroupsOutput{
			SecurityGroups: []ec2types.SecurityGroup{
				{
					GroupId:   awssdk.String("sg-12345678"),
					GroupName: awssdk.String("test-sg"),
				},
			},
		}, nil)

	clientEC2.On("DescribeSubnets", mock.Anything, mock.Anything, mock.Anything).
		Return(&ec2.DescribeSubnetsOutput{
			Subnets: []ec2types.Subnet{
				{
					SubnetId:         awssdk.String("subnet-12345678"),
					AvailabilityZone: awssdk.String("eu-central-1a"),
					Tags: []ec2types.Tag{
						{
							Key:   awssdk.String("kubernetes.io/cluster/aws:cluster-1"),
							Value: awssdk.String("owned"),
						},
					},
				},
			},
		}, nil)

	clientEC2.On("DescribeRouteTables", mock.Anything, mock.Anything, mock.Anything).
		Return(&ec2.DescribeRouteTablesOutput{
			RouteTables: []ec2types.RouteTable{
				{
					RouteTableId: awssdk.String("rtb-12345678"),
					Associations: []ec2types.RouteTableAssociation{
						{
							SubnetId: awssdk.String("subnet-12345678"),
						},
					},
				},
			},
		}, nil)

	clientCF.On("DescribeStacks", mock.Anything, mock.Anything, mock.Anything).
		Return(&cloudformation.DescribeStacksOutput{
			Stacks: []cftypes.Stack{
				{
					StackName:   awssdk.String("kube-ing-1"),
					StackStatus: cftypes.StackStatusCreateComplete,
					Tags: []cftypes.Tag{
						{
							Key:   awssdk.String("kubernetes.io/cluster/aws:cluster-1"),
							Value: awssdk.String("owned"),
						},
						{
							Key:   awssdk.String("ingress:certificate-arn/arn:test-cert-1"),
							Value: awssdk.String("0001-01-01T00:00:00Z"),
						},
					},
					Parameters: []cftypes.Parameter{
						{
							ParameterKey:   awssdk.String("LoadBalancerSecurityGroupParameter"),
							ParameterValue: awssdk.String("sg-12345678"),
						},
						{
							ParameterKey:   awssdk.String("IpAddressType"),
							ParameterValue: awssdk.String("ipv4"),
						},
						{
							ParameterKey:   awssdk.String("LoadBalancerSchemeParameter"),
							ParameterValue: awssdk.String("internet-facing"),
						},
						{
							ParameterKey:   awssdk.String("Type"),
							ParameterValue: awssdk.String(aws.LoadBalancerTypeNetwork),
						},
						{
							ParameterKey:   awssdk.String("HTTP2"),
							ParameterValue: awssdk.String("true"),
						},
						{
							ParameterKey:   awssdk.String("ListenerSslPolicyParameter"),
							ParameterValue: awssdk.String(aws.DefaultSslPolicy),
						},
					},
					Outputs: []cftypes.Output{
						{
							OutputKey:   awssdk.String("LoadBalancerDNSName"),
							OutputValue: awssdk.String("test-lb-1.amazonaws.com"),
						},
						{
							OutputKey:   awssdk.String("LoadBalancerARN"),
							OutputValue: awssdk.String("arn:test-lb-1"),
						},
					},
				},
			},
		}, nil)

	clientEC2.On("DescribeInstances", mock.Anything, mock.Anything, mock.Anything).
		Return(&ec2.DescribeInstancesOutput{
			Reservations: []ec2types.Reservation{},
		}, nil)

	clientASG.On("DescribeAutoScalingGroups", mock.Anything, mock.Anything, mock.Anything).
		Return(&autoscaling.DescribeAutoScalingGroupsOutput{
			AutoScalingGroups: []asgtypes.AutoScalingGroup{
				{
					AutoScalingGroupName:    awssdk.String("kube-ing-1"),
					LaunchConfigurationName: awssdk.String("kube-ing-1-launch-config"),
					MinSize:                 awssdk.Int32(1),
					MaxSize:                 awssdk.Int32(3),
					DesiredCapacity:         awssdk.Int32(2),
					AvailabilityZones:       []string{"eu-central-1a"},
					Tags: []asgtypes.TagDescription{
						{
							Key:   awssdk.String("kubernetes.io/cluster/aws:cluster-1"),
							Value: awssdk.String("owned"),
						},
					},
				},
			},
		}, nil)

	clientASG.On("DescribeLoadBalancerTargetGroups", mock.Anything, mock.Anything, mock.Anything).
		Return(&autoscaling.DescribeLoadBalancerTargetGroupsOutput{
			LoadBalancerTargetGroups: []asgtypes.LoadBalancerTargetGroupState{
				{
					LoadBalancerTargetGroupARN: awssdk.String("arn:tg-1"),
					State:                      awssdk.String("InService"),
				},
			},
		}, nil)

	clientELBv2.On("DescribeTargetGroups", mock.Anything, &elbv2.DescribeTargetGroupsInput{}, mock.Anything).
		Return(&elbv2.DescribeTargetGroupsOutput{
			TargetGroups: []elbv2types.TargetGroup{
				{
					TargetGroupArn: awssdk.String("arn:tg-1"),
					Protocol:       elbv2types.ProtocolEnumTcp,
					Port:           awssdk.Int32(80),
					VpcId:          awssdk.String("vpc-1"),
				},
			},
		}, nil)

	clientELBv2.On("DescribeTags", mock.Anything, &elbv2.DescribeTagsInput{ResourceArns: []string{"arn:tg-1"}}, mock.Anything).
		Return(&elbv2.DescribeTagsOutput{}, nil)

	a := &aws.Adapter{
		TargetCNI: &aws.TargetCNIconfig{Enabled: false},
	}
	a = a.WithCustomAutoScalingClient(clientASG).
		WithCustomEc2Client(clientEC2).
		WithCustomElbv2Client(clientELBv2).
		WithCustomCloudFormationClient(clientCF).
		WithNLBZoneAffinity(aws.DefaultZoneAffinity)

	a, err = a.UpdateManifest(context.Background(), "aws:cluster-1", "vpc-1")
	if err != nil {
		t.Fatal(err)
	}

	// Kubernetes mocks setup
	kubeAPI := &kubemock.API{}

	// Set up mock expectations for kubeAPI
	kubeAPI.On("ListResources").Return([]*kubernetes.Ingress{
		{
			ResourceType:     kubernetes.TypeIngress,
			Name:             "ingress-1",
			Namespace:        "default",
			Shared:           true,
			HTTP2:            true,
			ClusterLocal:     false,
			SSLPolicy:        aws.DefaultSslPolicy,
			IPAddressType:    aws.IPAddressTypeIPV4,
			SecurityGroup:    "sg-12345678",
			LoadBalancerType: aws.LoadBalancerTypeNetwork,
			Scheme:           string(elbv2types.LoadBalancerSchemeEnumInternetFacing),
			Hostnames:        []string{"ingress-1.test"},
		},
	}, nil)

	kubeAPI.On("UpdateIngressLoadBalancer", mock.Anything, mock.Anything).
		Return(nil).Maybe()

	// Worker setup
	firstRun = false
	t.Cleanup(func() { firstRun = true })

	suite.worker = &worker{
		awsAdapter:         a,
		kubeAPI:            kubeAPI,
		metrics:            newMetrics(),
		certsProvider:      certsProvider,
		certsPerALB:        10,
		certTTL:            1 * time.Hour,
		minLoadBalancerAge: minLoadBalancerAge,
	}
	suite.clientELBv2 = clientELBv2
	suite.kubeAPI = kubeAPI
}

func (suite *MinLoadBalancerAgeTestSuite) TestYoungLoadBalancer() {
	suite.clientELBv2.On("DescribeLoadBalancers", mock.Anything, mock.Anything, mock.Anything).
		Return(&elbv2.DescribeLoadBalancersOutput{
			LoadBalancers: []elbv2types.LoadBalancer{
				{
					LoadBalancerArn: awssdk.String("arn:test-lb-1"),
					Scheme:          elbv2types.LoadBalancerSchemeEnumInternetFacing,
					Type:            elbv2types.LoadBalancerTypeEnumNetwork,
					State:           &elbv2types.LoadBalancerState{Code: elbv2types.LoadBalancerStateEnumActive},
					CreatedTime:     awssdk.Time(time.Now().Add(-2 * time.Minute)),
				},
			},
		}, nil)

	problems := suite.worker.doWork(context.Background())

	suite.Empty(problems.Errors())

	suite.kubeAPI.AssertNotCalled(suite.T(), "UpdateIngressLoadBalancer", mock.Anything, mock.Anything)
}

func (suite *MinLoadBalancerAgeTestSuite) TestOldBalancer() {
	suite.clientELBv2.On("DescribeLoadBalancers", mock.Anything, mock.Anything, mock.Anything).
		Return(&elbv2.DescribeLoadBalancersOutput{
			LoadBalancers: []elbv2types.LoadBalancer{
				{
					LoadBalancerArn: awssdk.String("arn:test-lb-1"),
					Scheme:          elbv2types.LoadBalancerSchemeEnumInternetFacing,
					Type:            elbv2types.LoadBalancerTypeEnumNetwork,
					State:           &elbv2types.LoadBalancerState{Code: elbv2types.LoadBalancerStateEnumActive},
					CreatedTime:     awssdk.Time(time.Now().Add(-7 * time.Minute)),
				},
			},
		}, nil)

	problems := suite.worker.doWork(context.Background())

	suite.Empty(problems.Errors())

	ingressMatcher := mock.MatchedBy(func(ingress *kubernetes.Ingress) bool {
		return ingress.Namespace == "default" && ingress.Name == "ingress-1"
	})
	suite.kubeAPI.AssertCalled(suite.T(), "UpdateIngressLoadBalancer", ingressMatcher, "test-lb-1.amazonaws.com")
}
