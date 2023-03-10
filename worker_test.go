package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"net/http/httptest"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/service/autoscaling"
	cloudformation "github.com/mweagle/go-cloudformation"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	awsAdapter "github.com/zalando-incubator/kube-ingress-aws-controller/aws"
	"github.com/zalando-incubator/kube-ingress-aws-controller/certs"
	"github.com/zalando-incubator/kube-ingress-aws-controller/kubernetes"
	"github.com/zalando/skipper/dataclients/kubernetes/kubernetestest"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/zalando-incubator/kube-ingress-aws-controller/aws/fake"
)

// TODO(LThiesen): This should be extracted to another file.
// This is also just a copy of certs test so it might be useful
// to just distribute it in a separate package like aws/fake
type caSingleton struct {
	once      sync.Once
	err       error
	chainKey  *rsa.PrivateKey
	roots     *x509.CertPool
	chainCert *x509.Certificate
}

type mockedCertificateProvider struct {
	t  *testing.T
	ca caSingleton
}

func (m *mockedCertificateProvider) GetCertificates() ([]*certs.CertificateSummary, error) {
	tenYears := time.Hour * 24 * 365 * 10
	altNames := []string{"foo.bar.org"}
	arn := "DUMMY"
	notBefore := time.Now()
	notAfter := time.Now().Add(time.Hour * 24)

	m.ca.once.Do(func() {
		caKey, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			m.ca.err = fmt.Errorf("unable to generate CA key: %v", err)
			return
		}

		caCert := x509.Certificate{
			SerialNumber: big.NewInt(1),
			Subject: pkix.Name{
				Organization: []string{"Testing CA"},
			},
			NotBefore: time.Time{},
			NotAfter:  time.Now().Add(tenYears),

			KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
			BasicConstraintsValid: true,

			IsCA: true,
		}
		caBody, err := x509.CreateCertificate(rand.Reader, &caCert, &caCert, caKey.Public(), caKey)
		if err != nil {
			m.ca.err = fmt.Errorf("unable to generate CA certificate: %v", err)
			return
		}
		caReparsed, err := x509.ParseCertificate(caBody)
		if err != nil {
			m.ca.err = fmt.Errorf("unable to parse CA certificate: %v", err)
			return
		}
		m.ca.roots = x509.NewCertPool()
		m.ca.roots.AddCert(caReparsed)

		chainKey, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			m.ca.err = fmt.Errorf("unable to generate sub-CA key: %v", err)
			return
		}
		chainCert := x509.Certificate{
			SerialNumber: big.NewInt(2),
			Subject: pkix.Name{
				Organization: []string{"Testing Sub-CA"},
			},
			NotBefore: time.Time{},
			NotAfter:  time.Now().Add(tenYears),

			KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
			BasicConstraintsValid: true,

			IsCA: true,
		}
		chainBody, err := x509.CreateCertificate(rand.Reader, &chainCert, caReparsed, chainKey.Public(), caKey)
		if err != nil {
			m.ca.err = fmt.Errorf("unable to generate sub-CA certificate: %v", err)
			return
		}
		chainReparsed, err := x509.ParseCertificate(chainBody)
		if err != nil {
			m.ca.err = fmt.Errorf("unable to parse sub-CA certificate: %v", err)
			return
		}

		m.ca.chainKey = chainKey
		m.ca.chainCert = chainReparsed
	})

	certKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		require.NoErrorf(m.t, err, "unable to generate certificate key")
	}
	cert := x509.Certificate{
		SerialNumber: big.NewInt(3),
		DNSNames:     altNames,
		NotBefore:    notBefore,
		NotAfter:     notAfter,

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	body, err := x509.CreateCertificate(rand.Reader, &cert, m.ca.chainCert, certKey.Public(), m.ca.chainKey)
	if err != nil {
		require.NoErrorf(m.t, err, "unable to generate certificate")
	}
	reparsed, err := x509.ParseCertificate(body)
	if err != nil {
		require.NoErrorf(m.t, err, "unable to parse certificate")
	}

	c := certs.NewCertificate(arn, reparsed, []*x509.Certificate{m.ca.chainCert})
	return []*certs.CertificateSummary{c.WithRoots(m.ca.roots)}, nil
}

func TestResourceConversion(tt *testing.T) {
	clusterIDTagPrefix := "kubernetes.io/cluster/"
	clusterID := "aws:123:eu-central-1:kube-1"
	vpcID := "1"
	securityGroupID := "42"
	running := int64(16) // See https://github.com/aws/aws-sdk-go/blob/master/service/ec2/api.go, type InstanceState

	for _, scenario := range []struct {
		name           string
		ec2Responses   fake.Ec2MockOutputs
		asgResponses   fake.AutoscalingMockOutputs
		elbv2Responses fake.Elbv2MockOutputs
		cfResponses    fake.CfMockOutputs
		lbType         string
	}{
		{
			name: "simple_alb",
			ec2Responses: fake.Ec2MockOutputs{DescribeInstancesPages: fake.MockDescribeInstancesPagesOutput(
				nil,
				fake.TestInstance{
					Id:        "i0",
					Tags:      fake.Tags{"aws:autoscaling:groupName": "asg1", clusterIDTagPrefix + clusterID: "owned"},
					PrivateIp: "1.2.3.3",
					VpcId:     vpcID,
					State:     running,
				},
				fake.TestInstance{
					Id:        "i1",
					Tags:      fake.Tags{"aws:autoscaling:groupName": "asg1", clusterIDTagPrefix + clusterID: "owned"},
					PrivateIp: "1.2.3.4",
					VpcId:     vpcID,
					State:     running,
				},
				fake.TestInstance{
					Id:        "i2",
					Tags:      fake.Tags{"aws:autoscaling:groupName": "asg1", clusterIDTagPrefix + clusterID: "owned"},
					PrivateIp: "1.2.3.5",
					VpcId:     vpcID,
					State:     running,
				}),
				DescribeSecurityGroups: fake.R(fake.MockDescribeSecurityGroupsOutput(map[string]string{"id": securityGroupID}), nil),
				DescribeSubnets: fake.R(fake.MockDescribeSubnetsOutput(
					fake.TestSubnet{Id: "foo1", Name: "bar1", Az: "baz1", Tags: map[string]string{"kubernetes.io/role/elb": ""}}), nil),
				DescribeRouteTables: fake.R(fake.MockDescribeRouteTableOutput(
					fake.TestRouteTable{SubnetID: "foo1", GatewayIds: []string{"igw-foo1"}},
					fake.TestRouteTable{SubnetID: "mismatch", GatewayIds: []string{"igw-foo2"}, Main: true},
				), nil),
			},
			asgResponses: fake.AutoscalingMockOutputs{
				DescribeAutoScalingGroups: fake.R(fake.MockDescribeAutoScalingGroupOutput(map[string]fake.Asgtags{"asg1": {
					clusterIDTagPrefix + clusterID: "owned",
				}}), nil),
				DescribeLoadBalancerTargetGroups: fake.R(&autoscaling.DescribeLoadBalancerTargetGroupsOutput{
					LoadBalancerTargetGroups: []*autoscaling.LoadBalancerTargetGroupState{},
				}, nil),
				AttachLoadBalancerTargetGroups: fake.R(nil, nil),
			},
			elbv2Responses: fake.Elbv2MockOutputs{
				DescribeTargetGroups: fake.R(nil, nil),
				DescribeTags:         fake.R(nil, nil),
			},
			cfResponses: fake.CfMockOutputs{
				DescribeStackPages: fake.R(nil, nil),
				DescribeStacks:     fake.R(nil, nil),
				CreateStack:        fake.R(fake.MockCSOutput("42"), nil),
			},
			lbType: awsAdapter.LoadBalancerTypeApplication,
		},
		{
			name: "simple_nlb",
			ec2Responses: fake.Ec2MockOutputs{DescribeInstancesPages: fake.MockDescribeInstancesPagesOutput(
				nil,
				fake.TestInstance{
					Id:        "i0",
					Tags:      fake.Tags{"aws:autoscaling:groupName": "asg1", clusterIDTagPrefix + clusterID: "owned"},
					PrivateIp: "1.2.3.3",
					VpcId:     vpcID,
					State:     running,
				},
				fake.TestInstance{
					Id:        "i1",
					Tags:      fake.Tags{"aws:autoscaling:groupName": "asg1", clusterIDTagPrefix + clusterID: "owned"},
					PrivateIp: "1.2.3.4",
					VpcId:     vpcID,
					State:     running,
				},
				fake.TestInstance{
					Id:        "i2",
					Tags:      fake.Tags{"aws:autoscaling:groupName": "asg1", clusterIDTagPrefix + clusterID: "owned"},
					PrivateIp: "1.2.3.5",
					VpcId:     vpcID,
					State:     running,
				}),
				DescribeSecurityGroups: fake.R(fake.MockDescribeSecurityGroupsOutput(map[string]string{"id": securityGroupID}), nil),
				DescribeSubnets: fake.R(fake.MockDescribeSubnetsOutput(
					fake.TestSubnet{Id: "foo1", Name: "bar1", Az: "baz1", Tags: map[string]string{"kubernetes.io/role/elb": ""}}), nil),
				DescribeRouteTables: fake.R(fake.MockDescribeRouteTableOutput(
					fake.TestRouteTable{SubnetID: "foo1", GatewayIds: []string{"igw-foo1"}},
					fake.TestRouteTable{SubnetID: "mismatch", GatewayIds: []string{"igw-foo2"}, Main: true},
				), nil),
			},
			asgResponses: fake.AutoscalingMockOutputs{
				DescribeAutoScalingGroups: fake.R(fake.MockDescribeAutoScalingGroupOutput(map[string]fake.Asgtags{"asg1": {
					clusterIDTagPrefix + clusterID: "owned",
				}}), nil),
				DescribeLoadBalancerTargetGroups: fake.R(&autoscaling.DescribeLoadBalancerTargetGroupsOutput{
					LoadBalancerTargetGroups: []*autoscaling.LoadBalancerTargetGroupState{},
				}, nil),
				AttachLoadBalancerTargetGroups: fake.R(nil, nil),
			},
			elbv2Responses: fake.Elbv2MockOutputs{
				DescribeTargetGroups: fake.R(nil, nil),
				DescribeTags:         fake.R(nil, nil),
			},
			cfResponses: fake.CfMockOutputs{
				DescribeStackPages: fake.R(nil, nil),
				DescribeStacks:     fake.R(nil, nil),
				CreateStack:        fake.R(fake.MockCSOutput("42"), nil),
			},
			lbType: awsAdapter.LoadBalancerTypeNetwork,
		},
	} {
		tt.Run(scenario.name, func(t *testing.T) {
			b, err := os.ReadFile("./testdata/" + scenario.name + "/expected.cf")
			if err != nil {
				t.Fatal(err)
			}
			expected := string(b)

			ec2Client := &fake.MockEc2Client{Outputs: scenario.ec2Responses}
			asgClient := &fake.MockAutoScalingClient{Outputs: scenario.asgResponses}
			elbv2Client := &fake.MockElbv2Client{Outputs: scenario.elbv2Responses}
			cfClient := &fake.MockCloudFormationClient{Outputs: scenario.cfResponses}

			a := &awsAdapter.Adapter{
				TargetCNI: &awsAdapter.TargetCNIconfig{Enabled: false},
			}
			a = a.WithCustomAutoScalingClient(asgClient).
				WithCustomEc2Client(ec2Client).
				WithCustomElbv2Client(elbv2Client).
				WithCustomCloudFormationClient(cfClient)

			a, err = a.UpdateManifest(clusterID, vpcID)
			if err != nil {
				t.Error(err)
			}

			f, err := os.Open("./testdata/" + scenario.name + "/ing.yaml")
			if err != nil {
				t.Fatal(err)
			}

			api, err := kubernetestest.NewAPI(kubernetestest.TestAPIOptions{}, f)
			if err != nil {
				t.Fatal(err)
			}

			s := httptest.NewServer(api)
			defer s.Close()

			kubeConfig := kubernetes.InsecureConfig(s.URL)
			ingressClassFilterList := make([]string, 0)

			sslPolicy := "ELBSecurityPolicy-2016-08"
			clusterLocalDomain := ""
			ingressAPIVersion := kubernetes.IngressAPIVersionNetworkingV1

			k, err := kubernetes.NewAdapter(
				kubeConfig,
				ingressAPIVersion,
				ingressClassFilterList,
				securityGroupID,
				sslPolicy,
				scenario.lbType,
				clusterLocalDomain,
				true)
			if err != nil {
				t.Fatal(err)
			}
			log.SetLevel(log.DebugLevel)
			problems := doWork(&mockedCertificateProvider{t: t}, 10, time.Hour, a, k, "")
			if len(problems.Errors()) > 0 {
				t.Error(problems.Errors())
			}

			assert.Equal(
				t,
				expected,
				cfClient.GetLastGeneratedTemplate(),
			)
		})
	}
}

func TestAddIngress(tt *testing.T) {
	for _, test := range []struct {
		name            string
		loadBalancer    *loadBalancer
		certificateARNs []string
		ingress         *kubernetes.Ingress
		maxCerts        int
		added           bool
	}{
		{
			name: "scheme not matching",
			loadBalancer: &loadBalancer{
				scheme: "foo",
			},
			ingress: &kubernetes.Ingress{
				Scheme: "bar",
			},
			added: false,
		},
		{
			name: "security group not matching",
			loadBalancer: &loadBalancer{
				securityGroup: "foo",
			},
			ingress: &kubernetes.Ingress{
				SecurityGroup: "bar",
			},
			added: false,
		},
		{
			name: "ip address type not matching",
			loadBalancer: &loadBalancer{
				ipAddressType: awsAdapter.IPAddressTypeIPV4,
			},
			ingress: &kubernetes.Ingress{
				IPAddressType: awsAdapter.IPAddressTypeDualstack,
			},
			added: false,
		},
		{
			name: "don't add ingresses non-shared, non-owned load balancer",
			loadBalancer: &loadBalancer{
				stack: &awsAdapter.Stack{
					OwnerIngress: "foo/bar",
				},
			},
			ingress: &kubernetes.Ingress{
				Shared:    false,
				Namespace: "foo",
				Name:      "baz",
			},
			added: false,
		},
		{
			name: "don't add ingresses shared, to an owned load balancer",
			loadBalancer: &loadBalancer{
				stack: &awsAdapter.Stack{
					OwnerIngress: "foo/bar",
				},
			},
			ingress: &kubernetes.Ingress{
				Shared:    true,
				Namespace: "foo",
				Name:      "baz",
			},
			added: false,
		},
		{
			name: "add ingress to empty load balancer",
			loadBalancer: &loadBalancer{
				stack: &awsAdapter.Stack{},
				ingresses: map[string][]*kubernetes.Ingress{
					"foo": []*kubernetes.Ingress{
						{
							Shared: true,
						},
					},
				},
			},
			certificateARNs: []string{"foo", "bar", "baz"},
			ingress: &kubernetes.Ingress{
				Shared: true,
			},
			maxCerts: 5,
			added:    true,
		},
		{
			name: "fail adding when too many certs",
			loadBalancer: &loadBalancer{
				stack: &awsAdapter.Stack{},
				ingresses: map[string][]*kubernetes.Ingress{
					"foo": []*kubernetes.Ingress{
						{
							Shared: true,
						},
					},
				},
			},
			certificateARNs: []string{"foo", "bar"},
			ingress: &kubernetes.Ingress{
				Shared: true,
			},
			maxCerts: 1,
			added:    false,
		},
		{
			name: "with WAF ACL, but cluster local",
			loadBalancer: &loadBalancer{
				ingresses: make(map[string][]*kubernetes.Ingress),
			},
			ingress: &kubernetes.Ingress{
				WAFWebACLID:  "WAFZXX",
				Shared:       true,
				ClusterLocal: true,
			},
			added: true,
		},
		{
			name: "with WAF ACL id",
			loadBalancer: &loadBalancer{
				ingresses:   make(map[string][]*kubernetes.Ingress),
				wafWebACLID: "WAFZXX",
			},
			ingress: &kubernetes.Ingress{
				WAFWebACLID: "WAFZXX",
				Shared:      true,
			},
			added: true,
		},
		{
			name: "with WAF ACL id, to not matching LB",
			loadBalancer: &loadBalancer{
				ingresses: make(map[string][]*kubernetes.Ingress),
			},
			ingress: &kubernetes.Ingress{
				WAFWebACLID: "WAFZXX",
				Shared:      true,
			},
			added: false,
		},
		{
			name: "with WAF ACL id, to not matching LB, with different ACL id",
			loadBalancer: &loadBalancer{
				ingresses:   make(map[string][]*kubernetes.Ingress),
				wafWebACLID: "WAFZYY",
			},
			ingress: &kubernetes.Ingress{
				WAFWebACLID: "WAFZXX",
				Shared:      true,
			},
			added: false,
		},
		{
			name: "Adding/changing WAF, SG or TLS settings on non-shared LB should work",
			loadBalancer: &loadBalancer{
				ingresses: make(map[string][]*kubernetes.Ingress),
				stack: &awsAdapter.Stack{
					OwnerIngress: "foo/bar",
				},
				sslPolicy: "ELBSecurityPolicy-2016-08",
			},
			ingress: &kubernetes.Ingress{
				Name:          "bar",
				Namespace:     "foo",
				WAFWebACLID:   "WAFZXX",
				SecurityGroup: "bar",
				SSLPolicy:     "ELBSecurityPolicy-FS-2018-06",
				Shared:        false,
			},
			added: true,
		},
	} {
		tt.Run(test.name, func(t *testing.T) {
			assert.Equal(
				t,
				test.added,
				test.loadBalancer.addIngress(test.certificateARNs, test.ingress, test.maxCerts),
			)
		})
	}
}

func TestSortStacks(tt *testing.T) {
	testTime := time.Now()

	for _, test := range []struct {
		name           string
		stacks         []*awsAdapter.Stack
		expectedStacks []*awsAdapter.Stack
	}{
		{
			name:           "no stacks",
			stacks:         []*awsAdapter.Stack{},
			expectedStacks: []*awsAdapter.Stack{},
		},
		{
			name: "two unsorted stacks",
			stacks: []*awsAdapter.Stack{
				&awsAdapter.Stack{
					Name:            "foo",
					CertificateARNs: map[string]time.Time{},
				},
				&awsAdapter.Stack{
					Name: "bar",
					CertificateARNs: map[string]time.Time{
						"cert-arn": testTime,
					},
				},
			},
			expectedStacks: []*awsAdapter.Stack{
				&awsAdapter.Stack{
					Name: "bar",
					CertificateARNs: map[string]time.Time{
						"cert-arn": testTime,
					},
				},
				&awsAdapter.Stack{
					Name:            "foo",
					CertificateARNs: map[string]time.Time{},
				},
			},
		},
		{
			name: "two unsorted stacks with the same amount of certificates",
			stacks: []*awsAdapter.Stack{
				&awsAdapter.Stack{
					Name: "foo",
					CertificateARNs: map[string]time.Time{
						"different-cert-arn": testTime,
					},
				},
				&awsAdapter.Stack{
					Name: "bar",
					CertificateARNs: map[string]time.Time{
						"cert-arn": testTime,
					},
				},
			},
			expectedStacks: []*awsAdapter.Stack{
				&awsAdapter.Stack{
					Name: "bar",
					CertificateARNs: map[string]time.Time{
						"cert-arn": testTime,
					},
				},
				&awsAdapter.Stack{
					Name: "foo",
					CertificateARNs: map[string]time.Time{
						"different-cert-arn": testTime,
					},
				},
			},
		},
	} {
		tt.Run(test.name, func(t *testing.T) {
			sortStacks(test.stacks)

			assert.Equal(t, test.expectedStacks, test.stacks)
		})
	}
}

func TestCertificateSummaries(t *testing.T) {
	certificateSummaries := []*certs.CertificateSummary{&certs.CertificateSummary{}}

	certs := &Certificates{certificateSummaries: certificateSummaries}

	assert.Equal(t, certificateSummaries, certs.CertificateSummaries())
}

func TestCertificateExists(tt *testing.T) {
	existingCertificateARN := "existing-arn"
	nonExistingCertificateARN := "non-existing-arn"

	for _, test := range []struct {
		name                 string
		certificateSummaries []*certs.CertificateSummary
		exists               bool
	}{
		{
			name: "certificate is present",
			certificateSummaries: []*certs.CertificateSummary{
				certs.NewCertificate(
					existingCertificateARN,
					&x509.Certificate{},
					[]*x509.Certificate{{}},
				),
			},
			exists: true,
		},
		{
			name: "certificate is not present",
			certificateSummaries: []*certs.CertificateSummary{
				certs.NewCertificate(
					nonExistingCertificateARN,
					&x509.Certificate{},
					[]*x509.Certificate{{}},
				),
			},
			exists: false,
		},
	} {
		tt.Run(test.name, func(t *testing.T) {
			certs := NewCertificates(test.certificateSummaries)

			assert.Equal(t, test.exists, certs.CertificateExists(existingCertificateARN))
		})
	}
}

func TestGetAllLoadBalancers(tt *testing.T) {
	certTTL, _ := time.ParseDuration("90d")

	for _, test := range []struct {
		name          string
		stacks        []*awsAdapter.Stack
		certs         []*certs.CertificateSummary
		loadBalancers []*loadBalancer
	}{
		{
			name: "one stack",
			stacks: []*awsAdapter.Stack{
				{
					Scheme:        "foo",
					SecurityGroup: "sg-123456",
				},
			},
			certs: []*certs.CertificateSummary{},
			loadBalancers: []*loadBalancer{
				{
					existingStackCertificateARNs: map[string]time.Time{},
					securityGroup:                "sg-123456",
					scheme:                       "foo",
					shared:                       true,
					ingresses:                    map[string][]*kubernetes.Ingress{},
					certTTL:                      certTTL,
				},
			},
		},
		{
			name: "one stack with certificates",
			stacks: []*awsAdapter.Stack{
				{
					Scheme:        "foo",
					SecurityGroup: "sg-123456",
					CertificateARNs: map[string]time.Time{
						"cert-arn": {},
					},
				},
			},
			certs: []*certs.CertificateSummary{
				certs.NewCertificate(
					"cert-arn",
					&x509.Certificate{},
					[]*x509.Certificate{{}},
				),
			},
			loadBalancers: []*loadBalancer{
				{
					existingStackCertificateARNs: map[string]time.Time{
						"cert-arn": {},
					},
					securityGroup: "sg-123456",
					scheme:        "foo",
					shared:        true,
					ingresses: map[string][]*kubernetes.Ingress{
						"cert-arn": {},
					},
					certTTL: certTTL,
				},
			},
		},
		{
			name: "non existing certificate is not added to LB",
			stacks: []*awsAdapter.Stack{
				{
					Scheme:        "foo",
					SecurityGroup: "sg-123456",
					CertificateARNs: map[string]time.Time{
						"cert-arn": {},
					},
				},
			},
			certs: []*certs.CertificateSummary{},
			loadBalancers: []*loadBalancer{
				{
					existingStackCertificateARNs: map[string]time.Time{},
					securityGroup:                "sg-123456",
					scheme:                       "foo",
					shared:                       true,
					ingresses:                    map[string][]*kubernetes.Ingress{},
					certTTL:                      certTTL,
				},
			},
		},
	} {
		tt.Run(test.name, func(t *testing.T) {
			for i, loadBalancer := range test.loadBalancers {
				loadBalancer.stack = test.stacks[i]
			}

			assert.Equal(t, test.loadBalancers, getAllLoadBalancers(NewCertificates(test.certs), certTTL, test.stacks))
		})
	}
}

func TestGetCloudWatchAlarmConfigFromConfigMap(t *testing.T) {
	for _, test := range []struct {
		name     string
		cm       *kubernetes.ConfigMap
		expected awsAdapter.CloudWatchAlarmList
	}{
		{
			name:     "empty config map",
			cm:       &kubernetes.ConfigMap{},
			expected: awsAdapter.CloudWatchAlarmList{},
		},
		{
			name: "config map with one data key",
			cm: &kubernetes.ConfigMap{
				Data: map[string]string{
					"some-key": "- AlarmName: foo\n- AlarmName: bar\n",
				},
			},
			expected: awsAdapter.CloudWatchAlarmList{
				{AlarmName: cloudformation.String("foo")},
				{AlarmName: cloudformation.String("bar")},
			},
		},
		{
			name: "config map with multiple data keys",
			cm: &kubernetes.ConfigMap{
				Data: map[string]string{
					"some-other-key": "- AlarmName: baz\n",
					"some-key":       "- AlarmName: foo\n- AlarmName: bar\n",
				},
			},
			expected: awsAdapter.CloudWatchAlarmList{
				{AlarmName: cloudformation.String("foo")},
				{AlarmName: cloudformation.String("bar")},
				{AlarmName: cloudformation.String("baz")},
			},
		},
		{
			name: "config map with invalid yaml data",
			cm: &kubernetes.ConfigMap{
				Data: map[string]string{
					"some-key": "{",
				},
			},
			expected: awsAdapter.CloudWatchAlarmList{},
		},
		{
			name: "config map with partially invalid yaml data",
			cm: &kubernetes.ConfigMap{
				Data: map[string]string{
					"some-key":       "{",
					"some-other-key": "- AlarmName: baz\n",
				},
			},
			expected: awsAdapter.CloudWatchAlarmList{
				{AlarmName: cloudformation.String("baz")},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := getCloudWatchAlarmsFromConfigMap(test.cm)

			assert.Equal(t, test.expected, result)
		})
	}
}

func TestAttachCloudWatchAlarmsCopy(t *testing.T) {
	lbOne := &loadBalancer{scheme: "foo"}
	lbTwo := &loadBalancer{scheme: "bar"}

	lbs := []*loadBalancer{
		lbOne,
		lbTwo,
	}

	alarms := awsAdapter.CloudWatchAlarmList{
		{AlarmName: cloudformation.String("baz")},
	}

	attachCloudWatchAlarms(lbs, alarms)

	expected := []*loadBalancer{
		{scheme: "foo", cwAlarms: alarms},
		{scheme: "bar", cwAlarms: alarms},
	}

	require.Equal(t, expected, lbs)

	// This should not modify the alarms of lbTwo.
	lbOne.cwAlarms[0].AlarmName = cloudformation.String("qux")

	assert.Equal(t, cloudformation.String("baz"), lbTwo.cwAlarms[0].AlarmName)
}

func TestIsLBInSync(t *testing.T) {
	for _, test := range []struct {
		title  string
		lb     *loadBalancer
		expect bool
	}{{
		title: "not matching certificates",
		lb: &loadBalancer{
			ingresses: map[string][]*kubernetes.Ingress{
				"foo": []*kubernetes.Ingress{{}},
				"bar": []*kubernetes.Ingress{{}},
				"baz": []*kubernetes.Ingress{{}},
			},
			stack: &awsAdapter.Stack{
				CertificateARNs: map[string]time.Time{
					"foo": time.Time{},
					"bar": time.Time{},
				},
				CWAlarmConfigHash: awsAdapter.CloudWatchAlarmList{{}}.Hash(),
				WAFWebACLID:       "foo-bar-baz",
			},
			cwAlarms:    awsAdapter.CloudWatchAlarmList{{}},
			wafWebACLID: "foo-bar-baz",
		},
	}, {
		title: "not matching alarm",
		lb: &loadBalancer{
			ingresses: map[string][]*kubernetes.Ingress{
				"foo": []*kubernetes.Ingress{{}},
				"bar": []*kubernetes.Ingress{{}},
				"baz": []*kubernetes.Ingress{{}},
			},
			stack: &awsAdapter.Stack{
				CertificateARNs: map[string]time.Time{
					"foo": time.Time{},
					"bar": time.Time{},
					"baz": time.Time{},
				},
				CWAlarmConfigHash: awsAdapter.CloudWatchAlarmList{{}}.Hash(),
				WAFWebACLID:       "foo-bar-baz",
			},
			cwAlarms:    awsAdapter.CloudWatchAlarmList{{}, {}},
			wafWebACLID: "foo-bar-baz",
		},
	}, {
		title: "not matching WAF",
		lb: &loadBalancer{
			ingresses: map[string][]*kubernetes.Ingress{
				"foo": []*kubernetes.Ingress{{}},
				"bar": []*kubernetes.Ingress{{}},
				"baz": []*kubernetes.Ingress{{}},
			},
			stack: &awsAdapter.Stack{
				CertificateARNs: map[string]time.Time{
					"foo": time.Time{},
					"bar": time.Time{},
					"baz": time.Time{},
				},
				CWAlarmConfigHash: awsAdapter.CloudWatchAlarmList{{}}.Hash(),
				WAFWebACLID:       "foo-bar-baz",
			},
			cwAlarms:    awsAdapter.CloudWatchAlarmList{{}},
			wafWebACLID: "foo-bar",
		},
	}, {
		title: "in sync",
		lb: &loadBalancer{
			ingresses: map[string][]*kubernetes.Ingress{
				"foo": []*kubernetes.Ingress{{}},
				"bar": []*kubernetes.Ingress{{}},
				"baz": []*kubernetes.Ingress{{}},
			},
			stack: &awsAdapter.Stack{
				CertificateARNs: map[string]time.Time{
					"foo": time.Time{},
					"bar": time.Time{},
					"baz": time.Time{},
				},
				CWAlarmConfigHash: awsAdapter.CloudWatchAlarmList{{}}.Hash(),
				WAFWebACLID:       "foo-bar-baz",
			},
			cwAlarms:    awsAdapter.CloudWatchAlarmList{{}},
			wafWebACLID: "foo-bar-baz",
		},
		expect: true,
	}} {
		t.Run(test.title, func(t *testing.T) {
			require.Equal(t, test.expect, test.lb.inSync())
		})
	}
}

func TestMatchIngressesToLoadbalancers(t *testing.T) {
	defaultMaxCertsPerLB := 3
	defaultCerts := &certmock{
		summaries: []*certs.CertificateSummary{
			certs.NewCertificate(
				"foo",
				&x509.Certificate{
					DNSNames: []string{"foo.org", "bar.org"},
				},
				nil,
			),
		},
	}

	for _, test := range []struct {
		title         string
		certs         CertificatesFinder
		maxCertsPerLB int
		lbs           []*loadBalancer
		ingresses     []*kubernetes.Ingress
		validate      func(*testing.T, []*loadBalancer)
	}{{
		title: "only cluster local",
		validate: func(t *testing.T, lbs []*loadBalancer) {
			require.Equal(t, 1, len(lbs))
			require.True(t, lbs[0].clusterLocal)
		},
	}, {
		title: "cluster local and new",
		ingresses: []*kubernetes.Ingress{{
			Name: "foo-ingress",
			Hostnames: []string{
				"foo.org",
				"bar.org",
			},
		}},
		validate: func(t *testing.T, lbs []*loadBalancer) {
			require.Equal(t, 2, len(lbs))
			require.False(t, lbs[0].clusterLocal == lbs[1].clusterLocal)
		},
	}, {
		title: "existing load balancer",
		ingresses: []*kubernetes.Ingress{{
			Name: "foo-ingress",
			Hostnames: []string{
				"foo.org",
				"bar.org",
			},
			LoadBalancerType: awsAdapter.LoadBalancerTypeApplication,
			Shared:           true,
		}},
		lbs: []*loadBalancer{{
			loadBalancerType: awsAdapter.LoadBalancerTypeApplication,
			ingresses:        make(map[string][]*kubernetes.Ingress),
		}},
		validate: func(t *testing.T, lbs []*loadBalancer) {
			require.Equal(t, 2, len(lbs))
			for _, lb := range lbs {
				if lb.clusterLocal {
					continue
				}

				if len(lb.ingresses["foo"]) != 1 && lb.ingresses["foo"][0].Name != "foo-ingress" {
					t.Fatal("failed to match ingress to existing LB")
				}
			}
		},
	}, {
		title: "certificate by ARN",
		ingresses: []*kubernetes.Ingress{{
			Name:             "foo-ingress",
			CertificateARN:   "foo",
			LoadBalancerType: awsAdapter.LoadBalancerTypeApplication,
			Shared:           true,
		}},
		validate: func(t *testing.T, lbs []*loadBalancer) {
			require.Equal(t, 2, len(lbs))
			for _, lb := range lbs {
				if lb.clusterLocal {
					continue
				}

				if len(lb.ingresses["foo"]) != 1 && lb.ingresses["foo"][0].Name != "foo-ingress" {
					t.Fatal("failed to match ingress to existing LB")
				}
			}
		},
	}, {
		title: "certificate by ARN, does not exist",
		ingresses: []*kubernetes.Ingress{{
			Name:             "foo-ingress",
			CertificateARN:   "not-existing-arn",
			LoadBalancerType: awsAdapter.LoadBalancerTypeApplication,
			Shared:           true,
		}},
		validate: func(t *testing.T, lbs []*loadBalancer) {
			require.Equal(t, 1, len(lbs))
		},
	}, {
		title: "certificate by hostname, does not exist",
		ingresses: []*kubernetes.Ingress{{
			Name: "foo-ingress",
			Hostnames: []string{
				"baz.org",
			},
			LoadBalancerType: awsAdapter.LoadBalancerTypeApplication,
			Shared:           true,
		}},
		validate: func(t *testing.T, lbs []*loadBalancer) {
			require.Equal(t, 1, len(lbs))
		},
	}, {
		title: "multiple ingresses for the same LB",
		ingresses: []*kubernetes.Ingress{{
			Name:             "foo-ingress",
			LoadBalancerType: awsAdapter.LoadBalancerTypeApplication,
			Shared:           true,
			Hostnames: []string{
				"foo.org",
				"bar.org",
			},
		}, {
			Name:             "bar-ingress",
			LoadBalancerType: awsAdapter.LoadBalancerTypeApplication,
			Shared:           true,
			Hostnames: []string{
				"foo.org",
				"bar.org",
			},
		}},
		validate: func(t *testing.T, lbs []*loadBalancer) {
			require.Equal(t, 2, len(lbs))
			for _, lb := range lbs {
				if lb.clusterLocal {
					continue
				}

				require.Equal(t, 2, len(lb.ingresses["foo"]))
			}
		},
	}, {
		title: "multiple ingresses for the same LB, with WAF ID",
		ingresses: []*kubernetes.Ingress{{
			Name:             "foo-ingress",
			LoadBalancerType: awsAdapter.LoadBalancerTypeApplication,
			Shared:           true,
			Hostnames: []string{
				"foo.org",
				"bar.org",
			},
			WAFWebACLID: "foo-bar-baz",
		}, {
			Name:             "bar-ingress",
			LoadBalancerType: awsAdapter.LoadBalancerTypeApplication,
			Shared:           true,
			Hostnames: []string{
				"foo.org",
				"bar.org",
			},
			WAFWebACLID: "foo-bar-baz",
		}},
		validate: func(t *testing.T, lbs []*loadBalancer) {
			require.Equal(t, 2, len(lbs))
			for _, lb := range lbs {
				if lb.clusterLocal {
					continue
				}

				require.Equal(t, 2, len(lb.ingresses["foo"]))
			}
		},
	}, {
		title: "ingresses with different WAF IDs",
		ingresses: []*kubernetes.Ingress{{
			Name:             "foo-ingress",
			LoadBalancerType: awsAdapter.LoadBalancerTypeApplication,
			Shared:           true,
			Hostnames: []string{
				"foo.org",
				"bar.org",
			},
			WAFWebACLID: "foo-bar-baz",
		}, {
			Name:             "bar-ingress",
			LoadBalancerType: awsAdapter.LoadBalancerTypeApplication,
			Shared:           true,
			Hostnames: []string{
				"foo.org",
				"bar.org",
			},
			WAFWebACLID: "qux-quz-quuz",
		}},
		validate: func(t *testing.T, lbs []*loadBalancer) {
			require.Equal(t, 3, len(lbs))
			for _, lb := range lbs {
				if lb.clusterLocal {
					continue
				}

				require.Equal(t, 1, len(lb.ingresses["foo"]))
			}
		},
	}} {
		t.Run(test.title, func(t *testing.T) {
			var certs CertificatesFinder = defaultCerts
			if test.certs != nil {
				certs = test.certs
			}

			maxCertsPerLB := defaultMaxCertsPerLB
			if test.maxCertsPerLB > 0 {
				maxCertsPerLB = test.maxCertsPerLB
			}

			lbs := matchIngressesToLoadBalancers(test.lbs, certs, maxCertsPerLB, test.ingresses)
			test.validate(t, lbs)
		})
	}
}

func TestBuildModel(t *testing.T) {
	defaultMaxCertsPerLB := 3
	defaultCerts := &certmock{
		summaries: []*certs.CertificateSummary{
			certs.NewCertificate(
				"foo",
				&x509.Certificate{
					DNSNames: []string{"foo.org", "bar.org"},
				},
				nil,
			),
		},
	}

	const certTTL = time.Hour

	for _, test := range []struct {
		title         string
		certs         CertificatesFinder
		maxCertsPerLB int
		ingresses     []*kubernetes.Ingress
		stacks        []*awsAdapter.Stack
		alarms        awsAdapter.CloudWatchAlarmList
		globalWAFACL  string
		validate      func(*testing.T, []*loadBalancer)
	}{{
		title: "no alarm, no waf",
		ingresses: []*kubernetes.Ingress{{
			Name:             "foo-ingress",
			LoadBalancerType: awsAdapter.LoadBalancerTypeApplication,
			Shared:           true,
			Hostnames: []string{
				"foo.org",
				"bar.org",
			},
		}},
		validate: func(t *testing.T, lbs []*loadBalancer) {
			require.Equal(t, 2, len(lbs))
			for _, lb := range lbs {
				if lb.clusterLocal {
					continue
				}

				require.Equal(t, 0, len(lb.cwAlarms))
				require.Empty(t, lb.wafWebACLID)
			}
		},
	}, {
		title: "with cloudwatch alarm",
		ingresses: []*kubernetes.Ingress{{
			Name:             "foo-ingress",
			LoadBalancerType: awsAdapter.LoadBalancerTypeApplication,
			Shared:           true,
			Hostnames: []string{
				"foo.org",
				"bar.org",
			},
		}},
		alarms: awsAdapter.CloudWatchAlarmList{{}},
		validate: func(t *testing.T, lbs []*loadBalancer) {
			require.Equal(t, 2, len(lbs))
			for _, lb := range lbs {
				if lb.clusterLocal {
					continue
				}

				require.Equal(t, 1, len(lb.cwAlarms))
				require.Empty(t, lb.wafWebACLID)
			}
		},
	}, {
		title: "with global WAF",
		ingresses: []*kubernetes.Ingress{{
			Name:             "foo-ingress",
			LoadBalancerType: awsAdapter.LoadBalancerTypeApplication,
			Shared:           true,
			Hostnames: []string{
				"foo.org",
				"bar.org",
			},
		}},
		globalWAFACL: "foo-bar-baz",
		validate: func(t *testing.T, lbs []*loadBalancer) {
			require.Equal(t, 2, len(lbs))
			for _, lb := range lbs {
				if lb.clusterLocal {
					continue
				}

				require.Equal(t, 0, len(lb.cwAlarms))
				require.Equal(t, "foo-bar-baz", lb.wafWebACLID)
			}
		},
	}, {
		title: "with ingress defined WAF",
		ingresses: []*kubernetes.Ingress{{
			Name:             "foo-ingress",
			LoadBalancerType: awsAdapter.LoadBalancerTypeApplication,
			Shared:           true,
			Hostnames: []string{
				"foo.org",
				"bar.org",
			},
			WAFWebACLID: "foo-bar-baz",
		}},
		validate: func(t *testing.T, lbs []*loadBalancer) {
			require.Equal(t, 2, len(lbs))
			for _, lb := range lbs {
				if lb.clusterLocal {
					continue
				}

				require.Equal(t, 0, len(lb.cwAlarms))
				require.Equal(t, "foo-bar-baz", lb.wafWebACLID)
			}
		},
	}, {
		title: "with global and ingress defined WAF",
		ingresses: []*kubernetes.Ingress{{
			Name:             "foo-ingress",
			LoadBalancerType: awsAdapter.LoadBalancerTypeApplication,
			Shared:           true,
			Hostnames: []string{
				"foo.org",
				"bar.org",
			},
		}, {
			Name:             "foo-ingress",
			LoadBalancerType: awsAdapter.LoadBalancerTypeApplication,
			Shared:           true,
			Hostnames: []string{
				"foo.org",
				"bar.org",
			},
			WAFWebACLID: "foo-bar-baz",
		}},
		globalWAFACL: "qux-quz-quuz",
		validate: func(t *testing.T, lbs []*loadBalancer) {
			require.Equal(t, 3, len(lbs))
			var localFound, globalFound bool
			for _, lb := range lbs {
				if lb.clusterLocal {
					continue
				}

				require.Equal(t, 0, len(lb.cwAlarms))

				if lb.wafWebACLID == "foo-bar-baz" {
					localFound = true
				}

				if lb.wafWebACLID == "qux-quz-quuz" {
					globalFound = true
				}
			}

			require.True(t, localFound && globalFound)
		},
	}} {
		t.Run(test.title, func(t *testing.T) {
			var certs CertificatesFinder = defaultCerts
			if test.certs != nil {
				certs = test.certs
			}

			maxCertsPerLB := defaultMaxCertsPerLB
			if test.maxCertsPerLB > 0 {
				maxCertsPerLB = test.maxCertsPerLB
			}

			m := buildManagedModel(
				certs,
				maxCertsPerLB,
				certTTL,
				test.ingresses,
				test.stacks,
				test.alarms,
				test.globalWAFACL,
			)

			test.validate(t, m)
		})
	}
}

func TestDoWorkPanicReturnsProblem(t *testing.T) {
	problem := doWork(nil, 0, 0, nil, nil, "")

	require.NotNil(t, problem, "expected problem")
	require.Len(t, problem.Errors(), 1)
	require.Error(t, problem.Errors()[0], "panic caused by: runtime error: invalid memory address or nil pointer dereference")
}

func Test_cniEventHandler(t *testing.T) {
	t.Run("handles messages from channels and calls update functions", func(t *testing.T) {
		targetCNIcfg := &awsAdapter.TargetCNIconfig{TargetGroupCh: make(chan []string, 10)}
		targetCNIcfg.TargetGroupCh <- []string{"bar", "baz"}
		targetCNIcfg.TargetGroupCh <- []string{"foo"} // flush
		mutex := &sync.Mutex{}
		var targetSet, cniTGARNs []string
		mockTargetSetter := func(endpoints, cniTargetGroupARNs []string) error {
			mutex.Lock()
			targetSet = endpoints
			cniTGARNs = cniTargetGroupARNs
			mutex.Unlock()
			return nil
		}
		mockInformer := func(_ context.Context, c chan<- []string) error {
			c <- []string{"4.3.2.1", "4.3.2.1"}
			c <- []string{"1.2.3.4"} // flush
			return nil
		}
		ctx, cl := context.WithCancel(context.Background())
		defer cl()
		go cniEventHandler(ctx, targetCNIcfg, mockTargetSetter, mockInformer)

		require.Eventually(t, func() bool {
			mutex.Lock()
			defer mutex.Unlock()
			return reflect.DeepEqual(targetSet, []string{"1.2.3.4"})
		}, wait.ForeverTestTimeout, time.Millisecond*100)

		require.Eventually(t, func() bool {
			return reflect.DeepEqual(cniTGARNs, []string{"foo"})
		}, wait.ForeverTestTimeout, time.Millisecond*100)
	})
}

func TestCountByIngressType(t *testing.T) {
	ingresses := []*kubernetes.Ingress{
		&kubernetes.Ingress{ResourceType: kubernetes.TypeIngress},
		&kubernetes.Ingress{ResourceType: kubernetes.TypeIngress},
		&kubernetes.Ingress{ResourceType: kubernetes.TypeIngress},
		&kubernetes.Ingress{ResourceType: kubernetes.TypeRouteGroup},
		&kubernetes.Ingress{ResourceType: kubernetes.TypeRouteGroup},
	}

	counts := countByIngressType(ingresses)

	assert.Equal(t, 3, counts[kubernetes.TypeIngress])
	assert.Equal(t, 2, counts[kubernetes.TypeRouteGroup])
}
