package aws

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
)

func TestGetAutoScalingName(t *testing.T) {
	for _, test := range []struct {
		tags      map[string]string
		want      string
		wantError bool
	}{
		{map[string]string{autoScalingGroupNameTag: "foo"}, "foo", false},
		{map[string]string{autoScalingGroupNameTag: "foo", "bar": "baz", "zbr": "42"}, "foo", false},
		{map[string]string{"foo": "bar"}, "", true},
		{nil, "", true},
	} {
		t.Run(fmt.Sprintf("want-%s", test.want), func(t *testing.T) {
			got, err := getAutoScalingGroupName(test.tags)
			assertResultAndError(t, test.want, got, test.wantError, err)
		})
	}

}

func TestFindingSecurityGroup(t *testing.T) {
	for _, test := range []struct {
		name      string
		responses ec2MockOutputs
		want      *securityGroupDetails
		wantError bool
	}{
		{
			"success-find-sg",
			ec2MockOutputs{
				describeSecurityGroups: R(mockDSGOutput(map[string]string{"id": "foo"}), nil),
			},
			&securityGroupDetails{id: "id", name: "foo"},
			false,
		},
		{
			"fail-no-security-groups",
			ec2MockOutputs{describeSecurityGroups: R(mockDSGOutput(nil), nil)}, nil, true,
		},
		{
			"fail-with-aws-api-error",
			ec2MockOutputs{describeSecurityGroups: R(nil, dummyErr)}, nil, true,
		},
	} {
		t.Run(fmt.Sprintf("%v", test.name), func(t *testing.T) {
			ec2 := &mockEc2Client{outputs: test.responses}
			got, err := findSecurityGroupWithClusterID(ec2, "foo", "kube-ingress-aws-controller")
			assertResultAndError(t, test.want, got, test.wantError, err)
		})
	}
}

func TestInstanceDetails(t *testing.T) {
	for _, test := range []struct {
		given         instanceDetails
		wantClusterID string
	}{
		{
			given: instanceDetails{id: "this-should-be-fine", vpcID: "bar", tags: map[string]string{
				nameTag:                    "baz",
				clusterIDTagPrefix + "zbr": resourceLifecycleOwned,
			}},
			wantClusterID: "zbr",
		},
		{
			given: instanceDetails{id: "this-should-be-fine-legacy", vpcID: "bar", tags: map[string]string{
				nameTag:                    "baz",
				kubernetesClusterLegacyTag: "zbr",
			}},
			wantClusterID: "zbr",
		},
		{
			given: instanceDetails{id: "this-should-be-fine-new-plus-legacy", vpcID: "bar", tags: map[string]string{
				nameTag:                    "foo",
				kubernetesClusterLegacyTag: "bar",
				clusterIDTagPrefix + "baz": resourceLifecycleOwned,
			}},
			wantClusterID: "baz",
		},
		{
			given: instanceDetails{id: "missing-name-tag", vpcID: "bar", tags: map[string]string{
				clusterIDTagPrefix + "zbr": resourceLifecycleOwned,
			}},
			wantClusterID: "zbr",
		},
		{
			given: instanceDetails{id: "missing-cluster-id-tag", vpcID: "bar", tags: map[string]string{
				nameTag: "baz",
			}},
			wantClusterID: defaultClusterID,
		},
		{
			given:         instanceDetails{id: "missing-mgmt-tags", vpcID: "bar", tags: map[string]string{}},
			wantClusterID: defaultClusterID,
		},
		{
			given:         instanceDetails{id: "nil-mgmt-tags", vpcID: "bar"},
			wantClusterID: defaultClusterID,
		},
	} {
		t.Run(fmt.Sprintf("%v", test.given.id), func(t *testing.T) {
			if test.given.clusterID() != test.wantClusterID {
				t.Errorf("unexpected cluster ID. wanted %q, got %q", test.wantClusterID, test.given.clusterID())
			}
		})
	}
}

func TestGetInstanceDetails(t *testing.T) {
	for _, test := range []struct {
		name      string
		responses ec2MockOutputs
		want      *instanceDetails
		wantError bool
	}{
		{
			"success-call",
			ec2MockOutputs{describeInstances: R(mockDIOutput(
				testInstance{id: "foo", tags: tags{"bar": "baz"}, state: runningState},
			), nil)},
			&instanceDetails{id: "foo", tags: map[string]string{"bar": "baz"}, running: true},
			false,
		},
		{
			"failed-state-match",
			ec2MockOutputs{describeInstances: R(mockDIOutput(
				testInstance{id: "foo1", tags: tags{"bar": "baz"}, state: 0},
				testInstance{id: "foo2", tags: tags{"bar": "baz"}, state: 32}, // shutting-down
				testInstance{id: "foo2", tags: tags{"bar": "baz"}, state: 48}, // terminated includes running?!?
				testInstance{id: "foo2", tags: tags{"bar": "baz"}, state: 64}, // stopping
				testInstance{id: "foo2", tags: tags{"bar": "baz"}, state: 80}, // stopped includes running?!?
			), nil)},
			nil,
			true,
		},
		{
			"nothing-returned-from-aws-api",
			ec2MockOutputs{describeInstances: R(mockDIOutput(), nil)},
			nil,
			true,
		},
		{
			"aws-api-fail",
			ec2MockOutputs{describeInstances: R(nil, dummyErr)},
			nil,
			true,
		},
	} {
		t.Run(fmt.Sprintf("%v", test.name), func(t *testing.T) {
			ec2 := &mockEc2Client{outputs: test.responses}
			got, err := getInstanceDetails(ec2, "foo")
			assertResultAndError(t, test.want, got, test.wantError, err)
		})
	}
}

func TestGetSubnets(t *testing.T) {
	for _, test := range []struct {
		name      string
		responses ec2MockOutputs
		want      []*subnetDetails
		wantError bool
	}{
		{
			"success-call-nofilter",
			ec2MockOutputs{
				describeSubnets: R(mockDSOutput(
					testSubnet{id: "foo1", name: "bar1", az: "baz1", tags: map[string]string{elbRoleTagName: ""}},
					testSubnet{id: "foo2", name: "bar2", az: "baz2"},
				), nil),
				describeRouteTables: R(mockDRTOutput(
					testRouteTable{subnetID: "foo1", gatewayIds: []string{"igw-foo1"}},
					testRouteTable{subnetID: "mismatch", gatewayIds: []string{"igw-foo2"}, main: true},
				), nil),
			},
			[]*subnetDetails{
				{id: "foo1", availabilityZone: "baz1", public: true, tags: map[string]string{nameTag: "bar1", elbRoleTagName: ""}},
				{id: "foo2", availabilityZone: "baz2", public: true, tags: map[string]string{nameTag: "bar2"}},
			},
			false,
		},
		{
			"success-call-filtered",
			ec2MockOutputs{
				describeSubnets: R(mockDSOutput(
					testSubnet{id: "foo1", name: "bar1", az: "baz1", tags: map[string]string{elbRoleTagName: "", clusterIDTagPrefix + "bar": "shared"}},
					testSubnet{id: "foo2", name: "bar2", az: "baz2", tags: map[string]string{clusterIDTagPrefix + "bar": "shared"}},
				), nil),
				describeRouteTables: R(mockDRTOutput(
					testRouteTable{subnetID: "foo1", gatewayIds: []string{"igw-foo1"}},
					testRouteTable{subnetID: "mismatch", gatewayIds: []string{"igw-foo2"}, main: true},
				), nil),
			},
			[]*subnetDetails{
				{id: "foo1", availabilityZone: "baz1", public: true, tags: map[string]string{nameTag: "bar1", elbRoleTagName: "", clusterIDTagPrefix + "bar": "shared"}},
				{id: "foo2", availabilityZone: "baz2", public: true, tags: map[string]string{nameTag: "bar2", clusterIDTagPrefix + "bar": "shared"}},
			},
			false,
		},
		{
			"aws-sdk-failure-describing-subnets",
			ec2MockOutputs{describeSubnets: R(nil, dummyErr)}, nil, true,
		},
		{
			"aws-sdk-failure-describing-route-tables",
			ec2MockOutputs{
				describeSubnets: R(mockDSOutput(
					testSubnet{id: "foo1", name: "bar1", az: "baz1"},
					testSubnet{id: "foo2", name: "bar2", az: "baz2"},
				), nil),
				describeRouteTables: R(nil, dummyErr),
			}, nil, true,
		},
		{
			"failure-to-map-subnets",
			ec2MockOutputs{
				describeSubnets: R(mockDSOutput(
					testSubnet{id: "foo1", name: "bar1", az: "baz1"},
				), nil),
				describeRouteTables: R(mockDRTOutput(
					testRouteTable{subnetID: "x", gatewayIds: []string{"y"}},
				), nil),
			},
			nil, true,
		},
	} {
		t.Run(fmt.Sprintf("%v", test.name), func(t *testing.T) {
			ec2 := &mockEc2Client{outputs: test.responses}
			got, err := getSubnets(ec2, "foo", "bar")
			assertResultAndError(t, test.want, got, test.wantError, err)
		})
	}
}

func TestGetInstancesDetailsWithFilters(t *testing.T) {
	for _, test := range []struct {
		name      string
		input     []*ec2.Filter
		responses ec2MockOutputs
		want      map[string]*instanceDetails
		wantError bool
	}{
		{
			"success-call",
			[]*ec2.Filter{
				{
					Name: aws.String("tag:KubernetesCluster"),
					Values: []*string{
						aws.String("kube1"),
					},
				},
			},
			ec2MockOutputs{describeInstancesPages: mockDIPOutput(
				nil,
				testInstance{id: "foo1", tags: tags{"bar": "baz"}, privateIp: "1.2.3.4", vpcId: "1", state: 16},
				testInstance{id: "foo2", tags: tags{"bar": "baz"}, privateIp: "1.2.3.5", vpcId: "1", state: 32},
				testInstance{id: "foo3", tags: tags{"aaa": "zzz"}, privateIp: "1.2.3.6", vpcId: "1", state: 80},
			)},
			map[string]*instanceDetails{
				"foo1": &instanceDetails{id: "foo1", tags: map[string]string{"bar": "baz"}, ip: "1.2.3.4", vpcID: "1", running: true},
				"foo2": &instanceDetails{id: "foo2", tags: map[string]string{"bar": "baz"}, ip: "1.2.3.5", vpcID: "1", running: false},
				"foo3": &instanceDetails{id: "foo3", tags: map[string]string{"aaa": "zzz"}, ip: "1.2.3.6", vpcID: "1", running: false},
			},
			false,
		},
		{
			"success-empty-filters",
			[]*ec2.Filter{},
			ec2MockOutputs{describeInstancesPages: mockDIPOutput(
				nil,
				testInstance{id: "foo1", tags: tags{"bar": "baz"}, privateIp: "1.2.3.4", vpcId: "1", state: 16},
				testInstance{id: "foo3", tags: tags{"aaa": "zzz"}, privateIp: "1.2.3.6", vpcId: "1", state: 80},
			)},
			map[string]*instanceDetails{
				"foo1": &instanceDetails{id: "foo1", tags: map[string]string{"bar": "baz"}, ip: "1.2.3.4", vpcID: "1", running: true},
				"foo3": &instanceDetails{id: "foo3", tags: map[string]string{"aaa": "zzz"}, ip: "1.2.3.6", vpcID: "1", running: false},
			},
			false,
		},
		{
			"success-empty-response",
			[]*ec2.Filter{
				{
					Name: aws.String("vpc-id"),
					Values: []*string{
						aws.String("some-vpc"),
					},
				},
			},
			ec2MockOutputs{describeInstancesPages: mockDIPOutput(nil)},
			map[string]*instanceDetails{},
			false,
		},
		{
			"aws-api-fail",
			[]*ec2.Filter{
				{
					Name: aws.String("tag-key"),
					Values: []*string{
						aws.String("key1"),
					},
				},
			},
			ec2MockOutputs{describeInstancesPages: mockDIPOutput(dummyErr, testInstance{})},
			nil,
			true,
		},
	} {
		t.Run(fmt.Sprintf("%v", test.name), func(t *testing.T) {
			ec2 := &mockEc2Client{outputs: test.responses}
			got, err := getInstancesDetailsWithFilters(ec2, test.input)
			assertResultAndError(t, test.want, got, test.wantError, err)
		})
	}
}

func assertResultAndError(t *testing.T, want, got interface{}, wantError bool, err error) {
	if wantError {
		if err == nil {
			t.Error("wanted an error but call seemed to have succeeded")
		}
	} else {
		if err != nil {
			t.Fatal("unexpected error", err)
		}

		if !reflect.DeepEqual(want, got) {
			t.Errorf("unexpected result. wanted %+v, got %+v", want, got)
		}
	}
}
