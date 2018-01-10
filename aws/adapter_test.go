package aws

import (
	"sort"
	"testing"

	"fmt"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"reflect"
)

func TestRemoveObsoletteInstances(tt *testing.T) {
	a := Adapter{instancesDetails: map[string]*instanceDetails{
		"1.2.3.4": nil,
		"1.2.3.5": nil,
	}}
	a.removeObsoleteInstances([]string{"1.2.3.4", "1.2.3.5"})
	if len(a.instancesDetails) != 2 {
		tt.Errorf("unexpected change in instancesDetails - %q", a.instancesDetails)
	}
	a.removeObsoleteInstances([]string{"1.2.3.4"})
	if len(a.instancesDetails) != 1 {
		tt.Errorf("unexpected change in instancesDetails - %q", a.instancesDetails)
	}
	if _, ok := a.instancesDetails["1.2.3.4"]; !ok {
		tt.Errorf("1.2.3.4 not found in instancesDetails")
	}
}

func TestFetchMissingInstances(tt *testing.T) {
	a := Adapter{instancesDetails: map[string]*instanceDetails{}}
	for _, test := range []struct {
		name      string
		input     []string
		responses ec2MockOutputs
		wantError bool
	}{
		{
			"initial",
			[]string{"1.2.3.4", "1.2.3.5"},
			ec2MockOutputs{describeInstances: R(mockDIOutput(
				testInstance{id: "foo1", tags: tags{"aws:autoscaling:groupName": "asg"}, privateIp: "1.2.3.4", vpcId: "1"},
				testInstance{id: "foo2", tags: tags{"aws:autoscaling:groupName": "asg"}, privateIp: "1.2.3.5", vpcId: "1"},
			), nil)},
			false,
		},
		{
			"remove-last-instance",
			[]string{"1.2.3.4"},
			ec2MockOutputs{describeInstances: R(mockDIOutput(), nil)},
			false,
		},
		{
			"add-two-more-instances",
			[]string{"1.2.3.6", "1.2.3.4", "1.2.3.5"},
			ec2MockOutputs{describeInstances: R(mockDIOutput(
				testInstance{id: "foo2", tags: tags{"aws:autoscaling:groupName": "asg"}, privateIp: "1.2.3.5", vpcId: "1"},
				testInstance{id: "foo3", tags: tags{"aws:autoscaling:groupName": "asg"}, privateIp: "1.2.3.6", vpcId: "1"},
			), nil)},
			false,
		},
		{
			"remove-all-instances",
			[]string{},
			ec2MockOutputs{describeInstances: R(mockDIOutput(), nil)},
			false,
		},
		{
			"fail-fetching-instance",
			[]string{"1.2.3.7", "1.2.3.8"},
			ec2MockOutputs{describeInstances: R(mockDIOutput(
				testInstance{id: "foo3", tags: tags{"aws:autoscaling:groupName": "asg"}, privateIp: "1.2.3.7", vpcId: "1"},
			), nil)},
			true,
		},
	} {
		tt.Run(fmt.Sprintf("%v", test.name), func(t *testing.T) {
			ec2 := &mockEc2Client{outputs: test.responses}
			a.ec2 = ec2
			a.removeObsoleteInstances(test.input)
			err := a.fetchMissingInstances(test.input)
			if test.wantError && err == nil {
				t.Errorf("expected error, got nothing with input - %q", test.input)
			}
			if !test.wantError && err != nil {
				t.Errorf("unexpected error '%s' with input - %q", err, test.input)
			}
			if !test.wantError && err == nil {
				adapterIps := make([]string, 0, len(a.instancesDetails))
				for ip := range a.instancesDetails {
					adapterIps = append(adapterIps, ip)
				}
				sort.Strings(test.input)
				sort.Strings(adapterIps)
				if !reflect.DeepEqual(test.input, adapterIps) {
					t.Errorf("unexpected result. wanted %+v, got %+v", test.input, adapterIps)
				}
			}
		})
	}
}

func TestUpdateAutoScalingGroups(tt *testing.T) {
	a := Adapter{instancesDetails: map[string]*instanceDetails{}, autoScalingGroups: make(map[string]*autoScalingGroupDetails)}
	for _, test := range []struct {
		name         string
		input        []string
		ec2responses ec2MockOutputs
		asgresponses autoscalingMockOutputs
		wantAsgs     []string
		wantError    bool
	}{
		{
			"initial",
			[]string{"1.2.3.3", "1.2.3.4"},
			ec2MockOutputs{describeInstances: R(mockDIOutput(
				testInstance{id: "foo0", tags: tags{"aws:autoscaling:groupName": "asg1"}, privateIp: "1.2.3.3", vpcId: "1"},
				testInstance{id: "foo1", tags: tags{"aws:autoscaling:groupName": "asg1"}, privateIp: "1.2.3.4", vpcId: "1"},
			), nil)},
			autoscalingMockOutputs{
				describeAutoScalingGroups: R(mockDASGOutput(map[string]asgtags{"asg1": {"foo": "bar"}}), nil),
			},
			[]string{"asg1"},
			false,
		},
		{
			"add-node-same-asg",
			[]string{"1.2.3.4", "1.2.3.5"},
			ec2MockOutputs{describeInstances: R(mockDIOutput(
				testInstance{id: "foo2", tags: tags{"aws:autoscaling:groupName": "asg1"}, privateIp: "1.2.3.5", vpcId: "1"},
			), nil)},
			autoscalingMockOutputs{},
			[]string{"asg1"},
			false,
		},
		{
			"add-node-second-asg",
			[]string{"1.2.3.4", "1.2.3.5", "2.1.1.1"},
			ec2MockOutputs{describeInstances: R(mockDIOutput(
				testInstance{id: "bar1", tags: tags{"aws:autoscaling:groupName": "asg2"}, privateIp: "2.1.1.1", vpcId: "1"},
			), nil)},
			autoscalingMockOutputs{
				describeAutoScalingGroups: R(mockDASGOutput(map[string]asgtags{"asg2": {"foo": "baz"}}), nil),
			},
			[]string{"asg1", "asg2"},
			false,
		},
		{
			"add-another-node-second-asg",
			[]string{"1.2.3.4", "1.2.3.5", "2.1.1.1", "2.2.2.2"},
			ec2MockOutputs{describeInstances: R(mockDIOutput(
				testInstance{id: "bar2", tags: tags{"aws:autoscaling:groupName": "asg2"}, privateIp: "2.2.2.2", vpcId: "1"},
			), nil)},
			autoscalingMockOutputs{},
			[]string{"asg1", "asg2"},
			false,
		},
		{
			"add-node-third-asg",
			[]string{"1.2.3.4", "1.2.3.5", "2.1.1.1", "2.2.2.2", "3.1.1.1"},
			ec2MockOutputs{describeInstances: R(mockDIOutput(
				testInstance{id: "bar1", tags: tags{"aws:autoscaling:groupName": "asg3"}, privateIp: "3.1.1.1", vpcId: "1"},
			), nil)},
			autoscalingMockOutputs{
				describeAutoScalingGroups: R(mockDASGOutput(map[string]asgtags{"asg3": {"foo": "baz"}}), nil),
			},
			[]string{"asg1", "asg2", "asg3"},
			false,
		},
		{
			"add-node-without-asg",
			[]string{"1.2.3.4", "1.2.3.5", "2.1.1.1", "2.2.2.2", "3.1.1.1", "0.1.1.1"},
			ec2MockOutputs{describeInstances: R(mockDIOutput(
				testInstance{id: "bar1", tags: tags{"Name": "node"}, privateIp: "0.1.1.1", vpcId: "1"},
			), nil)},
			autoscalingMockOutputs{},
			[]string{"asg1", "asg2", "asg3"},
			false,
		},
		{
			"remove-third-asg-node",
			[]string{"1.2.3.4", "1.2.3.5", "2.1.1.1", "2.2.2.2", "0.1.1.1"},
			ec2MockOutputs{},
			autoscalingMockOutputs{},
			[]string{"asg1", "asg2"},
			false,
		},
		{
			"error-fetching-instance",
			[]string{"1.2.3.4", "1.2.3.5", "2.1.1.1", "2.2.2.2", "0.1.1.1", "0.2.2.2"},
			ec2MockOutputs{describeInstances: R(mockDIOutput(), nil)},
			autoscalingMockOutputs{},
			[]string{"asg1", "asg2"},
			true,
		},
		{
			"error-fetching-asg",
			[]string{"1.2.3.4", "1.2.3.5", "2.1.1.1", "2.2.2.2", "0.1.1.1", "0.2.2.2"},
			ec2MockOutputs{describeInstances: R(mockDIOutput(
				testInstance{id: "foo2", tags: tags{"aws:autoscaling:groupName": "none"}, privateIp: "0.2.2.2", vpcId: "1"},
			), nil)},
			autoscalingMockOutputs{
				describeAutoScalingGroups: R(mockDASGOutput(map[string]asgtags{"asg": {"foo": "baz"}}), nil),
			},
			[]string{"asg1", "asg2"},
			true,
		},
		{
			"add-back-third-asg",
			[]string{"1.2.3.4", "1.2.3.5", "2.1.1.1", "2.2.2.2", "3.1.1.1", "0.1.1.1"},
			ec2MockOutputs{describeInstances: R(mockDIOutput(
				testInstance{id: "bar1", tags: tags{"aws:autoscaling:groupName": "asg3"}, privateIp: "3.1.1.1", vpcId: "1"},
			), nil)},
			autoscalingMockOutputs{
				describeAutoScalingGroups: R(mockDASGOutput(map[string]asgtags{"asg3": {"foo": "baz"}}), nil),
			},
			[]string{"asg1", "asg2", "asg3"},
			false,
		},
		{
			"remove-all-except-first-asg",
			[]string{"1.2.3.4", "1.2.3.5"},
			ec2MockOutputs{describeInstances: R(mockDIOutput(), nil)},
			autoscalingMockOutputs{},
			[]string{"asg1"},
			false,
		},
		{
			"add-remove-simultaneously",
			[]string{"1.2.3.4", "0.1.1.1"},
			ec2MockOutputs{describeInstances: R(mockDIOutput(
				testInstance{id: "bar1", tags: tags{"Name": "node"}, privateIp: "0.1.1.1", vpcId: "1"},
			), nil)},
			autoscalingMockOutputs{},
			[]string{"asg1"},
			false,
		},
	} {
		tt.Run(fmt.Sprintf("%v", test.name), func(t *testing.T) {
			ec2 := &mockEc2Client{outputs: test.ec2responses}
			autoscaling := &mockAutoScalingClient{outputs: test.asgresponses}
			a.ec2 = ec2
			a.autoscaling = autoscaling
			tt.Log(test.name)
			err := a.UpdateAutoScalingGroups(test.input)
			if test.wantError && err == nil {
				t.Errorf("expected error, got nothing with input - %q", test.input)
			}
			if !test.wantError && err != nil {
				t.Errorf("unexpected error '%s' with input - %q", err, test.input)
			}
			if !test.wantError && err == nil {
				adapterIps := make([]string, 0, len(a.instancesDetails))
				for ip := range a.instancesDetails {
					adapterIps = append(adapterIps, ip)
				}
				asgs := make([]string, 0, len(a.autoScalingGroups))
				for name := range a.autoScalingGroups {
					asgs = append(asgs, name)
				}
				sort.Strings(test.input)
				sort.Strings(adapterIps)
				if !reflect.DeepEqual(test.input, adapterIps) {
					t.Errorf("unexpected instancesDetails result. wanted %+v, got %+v", test.input, adapterIps)
				}
				sort.Strings(test.wantAsgs)
				sort.Strings(asgs)
				if !reflect.DeepEqual(test.wantAsgs, asgs) {
					t.Errorf("unexpected autoScalingGroups result. wanted %+v, got %+v", test.wantAsgs, asgs)
				}
			}
		})
	}
}

func TestFindLBSubnets(tt *testing.T) {
	for _, test := range []struct {
		name            string
		subnets         []*subnetDetails
		scheme          string
		expectedSubnets []string
	}{
		{
			name: "should find two public subnets for public LB",
			subnets: []*subnetDetails{
				{
					availabilityZone: "a",
					public:           true,
					id:               "1",
				},
				{
					availabilityZone: "b",
					public:           true,
					id:               "2",
				},
			},
			scheme:          elbv2.LoadBalancerSchemeEnumInternetFacing,
			expectedSubnets: []string{"1", "2"},
		},
		{
			name: "should select first lexicographically subnet when two match a single zone",
			subnets: []*subnetDetails{
				{
					availabilityZone: "a",
					public:           true,
					id:               "2",
				},
				{
					availabilityZone: "a",
					public:           true,
					id:               "1",
				},
			},
			scheme:          elbv2.LoadBalancerSchemeEnumInternetFacing,
			expectedSubnets: []string{"1"},
		},
		{
			name: "should not use internal subnets for public LB",
			subnets: []*subnetDetails{
				{
					availabilityZone: "a",
					public:           false,
					id:               "2",
				},
			},
			scheme:          elbv2.LoadBalancerSchemeEnumInternetFacing,
			expectedSubnets: nil,
		},
		{
			name: "should prefer tagged subnet",
			subnets: []*subnetDetails{
				{
					availabilityZone: "a",
					public:           true,
					id:               "1",
				},
				{
					availabilityZone: "a",
					public:           true,
					id:               "2",
					tags: map[string]string{
						elbRoleTagName: "",
					},
				},
			},
			scheme:          elbv2.LoadBalancerSchemeEnumInternetFacing,
			expectedSubnets: []string{"2"},
		},
		{
			name: "should prefer tagged subnet (internal)",
			subnets: []*subnetDetails{
				{
					availabilityZone: "a",
					public:           false,
					id:               "1",
				},
				{
					availabilityZone: "a",
					public:           false,
					id:               "2",
					tags: map[string]string{
						internalELBRoleTagName: "",
					},
				},
			},
			scheme:          elbv2.LoadBalancerSchemeEnumInternal,
			expectedSubnets: []string{"2"},
		},
	} {
		tt.Run(test.name, func(t *testing.T) {
			a := &Adapter{
				manifest: &manifest{
					subnets: test.subnets,
				},
			}

			subnets := a.FindLBSubnets(test.scheme)

			if len(subnets) != len(test.expectedSubnets) {
				t.Errorf("unexpected number of subnets %d, expected %d", len(subnets), len(test.expectedSubnets))
			}

			// sort subnets so it's simpler to compare.
			sort.Strings(subnets)

			for i, subnet := range subnets {
				if subnet != test.expectedSubnets[i] {
					t.Errorf("expected subnet %v, got %v", test.expectedSubnets[i], subnet)
				}
			}
		})
	}
}
