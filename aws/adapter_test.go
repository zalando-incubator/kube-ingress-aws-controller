package aws

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2Types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zalando-incubator/kube-ingress-aws-controller/aws/fake"
)

func TestGenerateDefaultFilters(tt *testing.T) {
	for _, test := range []struct {
		name      string
		clusterId string
	}{
		{
			"empty-cluster-id",
			"",
		},
		{
			"set1",
			"test1",
		},
		{
			"set2",
			"test2+test2",
		},
		{
			"set3",
			"=  = ",
		},
	} {
		tt.Run(fmt.Sprintf("%v", test.name), func(t *testing.T) {
			tt.Log(test.name)
			filters := generateDefaultFilters(test.clusterId)
			if len(filters) != 2 {
				t.Errorf("generateDefaultFilters returned %d filters instead of 2", len(filters))
			}
			if aws.ToString(filters[0].Name) != "tag:"+clusterIDTagPrefix+test.clusterId {
				t.Errorf("generateDefaultFilters first filter has wrong name %s", aws.ToString(filters[0].Name))
			}
			if len(filters[0].Values) != 1 {
				t.Errorf("generateDefaultFilters first filter has %d values instead of 1", len(filters[0].Values))
			}
			if filters[0].Values[0] != resourceLifecycleOwned {
				t.Errorf("generateDefaultFilters first filter has wrong value %s", filters[0].Values[0])
			}
			if aws.ToString(filters[1].Name) != "tag-key" {
				t.Errorf("generateDefaultFilters second filter has wrong name %s", aws.ToString(filters[1].Name))
			}
			if len(filters[1].Values) != 1 {
				t.Errorf("generateDefaultFilters second filter has %d values instead of 1", len(filters[1].Values))
			}
			if filters[1].Values[0] != kubernetesNodeRoleTag {
				t.Errorf("generateDefaultFilters second filter has wrong value %s", filters[1].Values[0])
			}
		})
	}
}

func TestParseFilters(tt *testing.T) {
	for _, test := range []struct {
		name            string
		customFilter    *string
		clusterId       string
		expectedFilters []types.Filter
	}{
		{
			"no-custom-filter",
			nil,
			"cluster",
			[]types.Filter{
				{
					Name:   aws.String("tag:" + clusterIDTagPrefix + "cluster"),
					Values: []string{resourceLifecycleOwned},
				},
				{
					Name:   aws.String("tag-key"),
					Values: []string{kubernetesNodeRoleTag},
				},
			},
		},
		{
			"custom-filter1",
			aws.String("tag:Test=test"),
			"cluster",
			[]types.Filter{
				{
					Name:   aws.String("tag:Test"),
					Values: []string{"test"},
				},
			},
		},
		{
			"custom-filter2",
			aws.String("tag:Test=test vpc-id=id1,id2"),
			"cluster",
			[]types.Filter{
				{
					Name:   aws.String("tag:Test"),
					Values: []string{"test"},
				},
				{
					Name:   aws.String("vpc-id"),
					Values: []string{"id1", "id2"},
				},
			},
		},
		{
			"custom-filter3",
			aws.String("tag:Test=test tag:Test=test1,test2  tag-key=key1,key2,key3"),
			"cluster",
			[]types.Filter{
				{
					Name:   aws.String("tag:Test"),
					Values: []string{"test"},
				},
				{
					Name:   aws.String("tag:Test"),
					Values: []string{"test1", "test2"},
				},
				{
					Name:   aws.String("tag-key"),
					Values: []string{"key1", "key2", "key3"},
				},
			},
		},
		{
			"illegal1",
			aws.String("test"),
			"cluster",
			[]types.Filter{
				{
					Name:   aws.String("tag:" + clusterIDTagPrefix + "cluster"),
					Values: []string{resourceLifecycleOwned},
				},
				{
					Name:   aws.String("tag-key"),
					Values: []string{kubernetesNodeRoleTag},
				},
			},
		},
	} {
		tt.Run(fmt.Sprintf("%v", test.name), func(t *testing.T) {
			tt.Log(test.name)
			a := &Adapter{
				ec2Details: map[string]*instanceDetails{},
				manifest: &manifest{
					instance: &instanceDetails{
						tags: map[string]string{
							clusterIDTagPrefix + test.clusterId: resourceLifecycleOwned,
						},
					},
				},
			}
			if test.customFilter != nil {
				a = a.WithCustomFilter(*test.customFilter)
			}
			output := a.parseFilters(test.clusterId)
			if !reflect.DeepEqual(test.expectedFilters, output) {
				t.Errorf("unexpected result. wanted %v, got %v", test.expectedFilters, output)
			}
		})
	}
}

func TestFiltersString(tt *testing.T) {
	for _, test := range []struct {
		name    string
		filters []types.Filter
		str     string
	}{
		{
			"test1",
			[]types.Filter{
				{
					Name:   aws.String("tag:" + clusterIDTagPrefix + "cluster"),
					Values: []string{resourceLifecycleOwned},
				},
				{
					Name:   aws.String("tag-key"),
					Values: []string{kubernetesNodeRoleTag},
				},
			},
			"tag:" + clusterIDTagPrefix + "cluster=" + resourceLifecycleOwned + " tag-key=" + kubernetesNodeRoleTag,
		},
		{
			"test2",
			[]types.Filter{
				{
					Name:   aws.String("tag:Test"),
					Values: []string{"test"},
				},
			},
			"tag:Test=test",
		},
		{
			"custom-filter2",
			[]types.Filter{
				{
					Name:   aws.String("tag:Test"),
					Values: []string{"test"},
				},
				{
					Name:   aws.String("vpc-id"),
					Values: []string{"id1", "id2"},
				},
			},
			"tag:Test=test vpc-id=id1,id2",
		},
		{
			"custom-filter3",
			[]types.Filter{
				{
					Name:   aws.String("tag:Test"),
					Values: []string{"test"},
				},
				{
					Name:   aws.String("tag:Test"),
					Values: []string{"test1", "test2"},
				},
				{
					Name:   aws.String("tag-key"),
					Values: []string{"key1", "key2", "key3"},
				},
			},
			"tag:Test=test tag:Test=test1,test2 tag-key=key1,key2,key3",
		},
	} {
		tt.Run(fmt.Sprintf("%v", test.name), func(t *testing.T) {
			tt.Log(test.name)
			a := Adapter{manifest: &manifest{filters: test.filters}}
			if test.str != a.FiltersString() {
				t.Errorf("filter string validation failure. wanted %q, got %q", test.str, a.FiltersString())
			}
		})
	}
}

func TestUpdateAutoScalingGroupsAndInstances(tt *testing.T) {
	clusterID := "aws:123:eu-central-1:kube-1"
	a := Adapter{
		ec2Details:        map[string]*instanceDetails{},
		singleInstances:   make(map[string]*instanceDetails),
		obsoleteInstances: []string{},
		manifest: &manifest{
			instance: &instanceDetails{
				tags: map[string]string{
					clusterIDTagPrefix + clusterID: resourceLifecycleOwned,
				},
			},
		},
	}
	for _, test := range []struct {
		name                       string
		ec2responses               fake.EC2Outputs
		asgresponses               fake.ASGOutputs
		cacheSize                  int
		wantAsgs                   []string
		wantSingleInstances        []string
		wantRunningSingleInstances []string
		wantObsoleteInstances      []string
		wantError                  bool
	}{
		{
			"initial",
			fake.EC2Outputs{DescribeInstances: fake.R(fake.MockDescribeInstancesOutput(
				fake.TestInstance{Id: "foo0", Tags: fake.Tags{"aws:autoscaling:groupName": "asg1"}, PrivateIp: "1.2.3.3", VpcId: "1", State: runningState},
				fake.TestInstance{Id: "foo1", Tags: fake.Tags{"aws:autoscaling:groupName": "asg1"}, PrivateIp: "1.2.3.4", VpcId: "1", State: runningState},
			), nil)},
			fake.ASGOutputs{
				DescribeAutoScalingGroups: fake.R(fake.MockDescribeAutoScalingGroupOutput(map[string]fake.ASGtags{"asg1": {
					"foo":                          "bar",
					clusterIDTagPrefix + clusterID: resourceLifecycleOwned,
				}}), nil),
			},
			2,
			[]string{"asg1"},
			[]string{},
			[]string{},
			[]string{},
			false,
		},
		{
			"add-node-same-asg",
			fake.EC2Outputs{DescribeInstances: fake.R(fake.MockDescribeInstancesOutput(
				fake.TestInstance{Id: "foo0", Tags: fake.Tags{"aws:autoscaling:groupName": "asg1"}, PrivateIp: "1.2.3.3", VpcId: "1", State: runningState},
				fake.TestInstance{Id: "foo1", Tags: fake.Tags{"aws:autoscaling:groupName": "asg1"}, PrivateIp: "1.2.3.4", VpcId: "1", State: 0},
				fake.TestInstance{Id: "foo2", Tags: fake.Tags{"aws:autoscaling:groupName": "asg1"}, PrivateIp: "1.2.3.5", VpcId: "1", State: runningState},
			), nil)},
			fake.ASGOutputs{
				DescribeAutoScalingGroups: fake.R(fake.MockDescribeAutoScalingGroupOutput(map[string]fake.ASGtags{"asg1": {
					"foo":                          "bar",
					clusterIDTagPrefix + clusterID: resourceLifecycleOwned,
				}}), nil),
			},
			3,
			[]string{"asg1"},
			[]string{},
			[]string{},
			[]string{},
			false,
		},
		{
			"add-node-second-asg",
			fake.EC2Outputs{DescribeInstances: fake.R(fake.MockDescribeInstancesOutput(
				fake.TestInstance{Id: "foo0", Tags: fake.Tags{"aws:autoscaling:groupName": "asg1"}, PrivateIp: "1.2.3.3", VpcId: "1", State: runningState},
				fake.TestInstance{Id: "foo1", Tags: fake.Tags{"aws:autoscaling:groupName": "asg1"}, PrivateIp: "1.2.3.4", VpcId: "1", State: runningState},
				fake.TestInstance{Id: "foo2", Tags: fake.Tags{"aws:autoscaling:groupName": "asg1"}, PrivateIp: "1.2.3.5", VpcId: "1", State: runningState},
				fake.TestInstance{Id: "bar1", Tags: fake.Tags{"aws:autoscaling:groupName": "asg2"}, PrivateIp: "2.1.1.1", VpcId: "1", State: runningState},
			), nil)},
			fake.ASGOutputs{
				DescribeAutoScalingGroups: fake.R(fake.MockDescribeAutoScalingGroupOutput(map[string]fake.ASGtags{
					"asg1": {
						"foo":                          "bar",
						clusterIDTagPrefix + clusterID: resourceLifecycleOwned,
					},
					"asg2": {
						"foo":                          "bar",
						clusterIDTagPrefix + clusterID: resourceLifecycleOwned,
					}}), nil),
			},
			4,
			[]string{"asg1", "asg2"},
			[]string{},
			[]string{},
			[]string{},
			false,
		},
		{
			"add-another-node-second-asg",
			fake.EC2Outputs{DescribeInstances: fake.R(fake.MockDescribeInstancesOutput(
				fake.TestInstance{Id: "foo0", Tags: fake.Tags{"aws:autoscaling:groupName": "asg1"}, PrivateIp: "1.2.3.3", VpcId: "1", State: runningState},
				fake.TestInstance{Id: "foo1", Tags: fake.Tags{"aws:autoscaling:groupName": "asg1"}, PrivateIp: "1.2.3.4", VpcId: "1", State: runningState},
				fake.TestInstance{Id: "foo2", Tags: fake.Tags{"aws:autoscaling:groupName": "asg1"}, PrivateIp: "1.2.3.5", VpcId: "1", State: runningState},
				fake.TestInstance{Id: "bar1", Tags: fake.Tags{"aws:autoscaling:groupName": "asg2"}, PrivateIp: "2.1.1.1", VpcId: "1", State: runningState},
				fake.TestInstance{Id: "bar2", Tags: fake.Tags{"aws:autoscaling:groupName": "asg2"}, PrivateIp: "2.2.2.2", VpcId: "1", State: runningState},
			), nil)},
			fake.ASGOutputs{
				DescribeAutoScalingGroups: fake.R(fake.MockDescribeAutoScalingGroupOutput(map[string]fake.ASGtags{
					"asg1": {
						"foo":                          "bar",
						clusterIDTagPrefix + clusterID: resourceLifecycleOwned,
					},
					"asg2": {
						"foo":                          "bar",
						clusterIDTagPrefix + clusterID: resourceLifecycleOwned,
					}}), nil),
			},
			5,
			[]string{"asg1", "asg2"},
			[]string{},
			[]string{},
			[]string{},
			false,
		},
		{
			"add-node-third-asg",
			fake.EC2Outputs{DescribeInstances: fake.R(fake.MockDescribeInstancesOutput(
				fake.TestInstance{Id: "foo0", Tags: fake.Tags{"aws:autoscaling:groupName": "asg1"}, PrivateIp: "1.2.3.3", VpcId: "1", State: runningState},
				fake.TestInstance{Id: "foo1", Tags: fake.Tags{"aws:autoscaling:groupName": "asg1"}, PrivateIp: "1.2.3.4", VpcId: "1", State: runningState},
				fake.TestInstance{Id: "foo2", Tags: fake.Tags{"aws:autoscaling:groupName": "asg1"}, PrivateIp: "1.2.3.5", VpcId: "1", State: runningState},
				fake.TestInstance{Id: "bar1", Tags: fake.Tags{"aws:autoscaling:groupName": "asg2"}, PrivateIp: "2.1.1.1", VpcId: "1", State: runningState},
				fake.TestInstance{Id: "bar2", Tags: fake.Tags{"aws:autoscaling:groupName": "asg2"}, PrivateIp: "2.2.2.2", VpcId: "1", State: runningState},
				fake.TestInstance{Id: "baz1", Tags: fake.Tags{"aws:autoscaling:groupName": "asg3"}, PrivateIp: "3.1.1.1", VpcId: "1", State: runningState},
			), nil)},
			fake.ASGOutputs{
				DescribeAutoScalingGroups: fake.R(fake.MockDescribeAutoScalingGroupOutput(map[string]fake.ASGtags{
					"asg1": {
						"foo":                          "bar",
						clusterIDTagPrefix + clusterID: resourceLifecycleOwned,
					},
					"asg2": {
						"foo":                          "bar",
						clusterIDTagPrefix + clusterID: resourceLifecycleOwned,
					},
					"asg3": {
						"foo":                          "bar",
						clusterIDTagPrefix + clusterID: resourceLifecycleOwned,
					}}), nil),
			},
			6,
			[]string{"asg1", "asg2", "asg3"},
			[]string{},
			[]string{},
			[]string{},
			false,
		},
		{
			"add-node-without-asg",
			fake.EC2Outputs{DescribeInstances: fake.R(fake.MockDescribeInstancesOutput(
				fake.TestInstance{Id: "foo0", Tags: fake.Tags{"aws:autoscaling:groupName": "asg1"}, PrivateIp: "1.2.3.3", VpcId: "1", State: runningState},
				fake.TestInstance{Id: "foo1", Tags: fake.Tags{"aws:autoscaling:groupName": "asg1"}, PrivateIp: "1.2.3.4", VpcId: "1", State: runningState},
				fake.TestInstance{Id: "foo2", Tags: fake.Tags{"aws:autoscaling:groupName": "asg1"}, PrivateIp: "1.2.3.5", VpcId: "1", State: runningState},
				fake.TestInstance{Id: "bar1", Tags: fake.Tags{"aws:autoscaling:groupName": "asg2"}, PrivateIp: "2.1.1.1", VpcId: "1", State: runningState},
				fake.TestInstance{Id: "bar2", Tags: fake.Tags{"aws:autoscaling:groupName": "asg2"}, PrivateIp: "2.2.2.2", VpcId: "1", State: runningState},
				fake.TestInstance{Id: "baz1", Tags: fake.Tags{"aws:autoscaling:groupName": "asg3"}, PrivateIp: "3.1.1.1", VpcId: "1", State: runningState},
				fake.TestInstance{Id: "sgl1", Tags: fake.Tags{"Name": "node1"}, PrivateIp: "0.1.1.1", VpcId: "1", State: runningState},
			), nil)},
			fake.ASGOutputs{
				DescribeAutoScalingGroups: fake.R(fake.MockDescribeAutoScalingGroupOutput(map[string]fake.ASGtags{
					"asg1": {
						"foo":                          "bar",
						clusterIDTagPrefix + clusterID: resourceLifecycleOwned,
					},
					"asg2": {
						"foo":                          "bar",
						clusterIDTagPrefix + clusterID: resourceLifecycleOwned,
					},
					"asg3": {
						"foo":                          "bar",
						clusterIDTagPrefix + clusterID: resourceLifecycleOwned,
					}}), nil),
			},
			7,
			[]string{"asg1", "asg2", "asg3"},
			[]string{"sgl1"},
			[]string{"sgl1"},
			[]string{},
			false,
		},
		{
			"add-stopped-node-without-asg",
			fake.EC2Outputs{DescribeInstances: fake.R(fake.MockDescribeInstancesOutput(
				fake.TestInstance{Id: "foo0", Tags: fake.Tags{"aws:autoscaling:groupName": "asg1"}, PrivateIp: "1.2.3.3", VpcId: "1", State: runningState},
				fake.TestInstance{Id: "foo1", Tags: fake.Tags{"aws:autoscaling:groupName": "asg1"}, PrivateIp: "1.2.3.4", VpcId: "1", State: runningState},
				fake.TestInstance{Id: "foo2", Tags: fake.Tags{"aws:autoscaling:groupName": "asg1"}, PrivateIp: "1.2.3.5", VpcId: "1", State: runningState},
				fake.TestInstance{Id: "bar1", Tags: fake.Tags{"aws:autoscaling:groupName": "asg2"}, PrivateIp: "2.1.1.1", VpcId: "1", State: runningState},
				fake.TestInstance{Id: "bar2", Tags: fake.Tags{"aws:autoscaling:groupName": "asg2"}, PrivateIp: "2.2.2.2", VpcId: "1", State: runningState},
				fake.TestInstance{Id: "baz1", Tags: fake.Tags{"aws:autoscaling:groupName": "asg3"}, PrivateIp: "3.1.1.1", VpcId: "1", State: runningState},
				fake.TestInstance{Id: "sgl1", Tags: fake.Tags{"Name": "node1"}, PrivateIp: "0.1.1.1", VpcId: "1", State: runningState},
				fake.TestInstance{Id: "sgl2", Tags: fake.Tags{"Name": "node2"}, PrivateIp: "0.1.1.2", VpcId: "1", State: stoppedState},
			), nil)},
			fake.ASGOutputs{
				DescribeAutoScalingGroups: fake.R(fake.MockDescribeAutoScalingGroupOutput(map[string]fake.ASGtags{
					"asg1": {
						"foo":                          "bar",
						clusterIDTagPrefix + clusterID: resourceLifecycleOwned,
					},
					"asg2": {
						"foo":                          "bar",
						clusterIDTagPrefix + clusterID: resourceLifecycleOwned,
					},
					"asg3": {
						"foo":                          "bar",
						clusterIDTagPrefix + clusterID: resourceLifecycleOwned,
					}}), nil),
			},
			8,
			[]string{"asg1", "asg2", "asg3"},
			[]string{"sgl1", "sgl2"},
			[]string{"sgl1"},
			[]string{},
			false,
		},
		{
			"remove-third-asg-node-and-stopped-instance",
			fake.EC2Outputs{DescribeInstances: fake.R(fake.MockDescribeInstancesOutput(
				fake.TestInstance{Id: "foo0", Tags: fake.Tags{"aws:autoscaling:groupName": "asg1"}, PrivateIp: "1.2.3.3", VpcId: "1", State: runningState},
				fake.TestInstance{Id: "foo1", Tags: fake.Tags{"aws:autoscaling:groupName": "asg1"}, PrivateIp: "1.2.3.4", VpcId: "1", State: runningState},
				fake.TestInstance{Id: "foo2", Tags: fake.Tags{"aws:autoscaling:groupName": "asg1"}, PrivateIp: "1.2.3.5", VpcId: "1", State: runningState},
				fake.TestInstance{Id: "bar1", Tags: fake.Tags{"aws:autoscaling:groupName": "asg2"}, PrivateIp: "2.1.1.1", VpcId: "1", State: runningState},
				fake.TestInstance{Id: "bar2", Tags: fake.Tags{"aws:autoscaling:groupName": "asg2"}, PrivateIp: "2.2.2.2", VpcId: "1", State: runningState},
				fake.TestInstance{Id: "sgl1", Tags: fake.Tags{"Name": "node1"}, PrivateIp: "0.1.1.1", VpcId: "1", State: runningState},
			), nil)},
			fake.ASGOutputs{
				DescribeAutoScalingGroups: fake.R(fake.MockDescribeAutoScalingGroupOutput(map[string]fake.ASGtags{
					"asg1": {
						"foo":                          "bar",
						clusterIDTagPrefix + clusterID: resourceLifecycleOwned,
					},
					"asg2": {
						"foo":                          "bar",
						clusterIDTagPrefix + clusterID: resourceLifecycleOwned,
					}}), nil),
			},
			6,
			[]string{"asg1", "asg2"},
			[]string{"sgl1"},
			[]string{"sgl1"},
			[]string{"sgl2"},
			false,
		},
		{
			"error-fetching-instance",
			fake.EC2Outputs{
				DescribeInstances: fake.R(fake.MockDescribeInstancesOutput(
					fake.TestInstance{},
				), fake.ErrDummy),
			},
			fake.ASGOutputs{},
			6,
			[]string{"asg1", "asg2"},
			[]string{"sgl1"},
			[]string{"sgl1"},
			[]string{"sgl2"},
			true,
		},
		{
			"error-fetching-asg",
			fake.EC2Outputs{DescribeInstances: fake.R(fake.MockDescribeInstancesOutput(
				fake.TestInstance{Id: "foo0", Tags: fake.Tags{"aws:autoscaling:groupName": "asg1"}, PrivateIp: "1.2.3.3", VpcId: "1", State: runningState},
				fake.TestInstance{Id: "foo1", Tags: fake.Tags{"aws:autoscaling:groupName": "asg1"}, PrivateIp: "1.2.3.4", VpcId: "1", State: runningState},
				fake.TestInstance{Id: "foo2", Tags: fake.Tags{"aws:autoscaling:groupName": "asg1"}, PrivateIp: "1.2.3.5", VpcId: "1", State: runningState},
				fake.TestInstance{Id: "bar1", Tags: fake.Tags{"aws:autoscaling:groupName": "asg2"}, PrivateIp: "2.1.1.1", VpcId: "1", State: runningState},
				fake.TestInstance{Id: "bar2", Tags: fake.Tags{"aws:autoscaling:groupName": "asg2"}, PrivateIp: "2.2.2.2", VpcId: "1", State: runningState},
				fake.TestInstance{Id: "sgl1", Tags: fake.Tags{"Name": "node1"}, PrivateIp: "0.1.1.1", VpcId: "1", State: runningState},
				fake.TestInstance{Id: "fail", Tags: fake.Tags{"aws:autoscaling:groupName": "none"}, PrivateIp: "0.2.2.2", VpcId: "1", State: runningState},
			), nil)},
			fake.ASGOutputs{
				DescribeAutoScalingGroups: fake.R(fake.MockDescribeAutoScalingGroupOutput(map[string]fake.ASGtags{
					"asg1": {
						"foo":                          "bar",
						clusterIDTagPrefix + clusterID: resourceLifecycleOwned,
					},
					"asg2": {
						"foo":                          "bar",
						clusterIDTagPrefix + clusterID: resourceLifecycleOwned,
					}}), nil),
			},
			7,
			[]string{"asg1", "asg2"},
			[]string{"sgl1"},
			[]string{"sgl1"},
			[]string{"sgl2"},
			false,
		},
		{
			"add-back-third-asg",
			fake.EC2Outputs{DescribeInstances: fake.R(fake.MockDescribeInstancesOutput(
				fake.TestInstance{Id: "foo0", Tags: fake.Tags{"aws:autoscaling:groupName": "asg1"}, PrivateIp: "1.2.3.3", VpcId: "1", State: runningState},
				fake.TestInstance{Id: "foo1", Tags: fake.Tags{"aws:autoscaling:groupName": "asg1"}, PrivateIp: "1.2.3.4", VpcId: "1", State: runningState},
				fake.TestInstance{Id: "foo2", Tags: fake.Tags{"aws:autoscaling:groupName": "asg1"}, PrivateIp: "1.2.3.5", VpcId: "1", State: runningState},
				fake.TestInstance{Id: "bar1", Tags: fake.Tags{"aws:autoscaling:groupName": "asg2"}, PrivateIp: "2.1.1.1", VpcId: "1", State: runningState},
				fake.TestInstance{Id: "bar2", Tags: fake.Tags{"aws:autoscaling:groupName": "asg2"}, PrivateIp: "2.2.2.2", VpcId: "1", State: runningState},
				fake.TestInstance{Id: "baz1", Tags: fake.Tags{"aws:autoscaling:groupName": "asg3"}, PrivateIp: "3.1.1.1", VpcId: "1", State: runningState},
				fake.TestInstance{Id: "sgl1", Tags: fake.Tags{"Name": "node1"}, PrivateIp: "0.1.1.1", VpcId: "1", State: runningState},
			), nil)},
			fake.ASGOutputs{
				DescribeAutoScalingGroups: fake.R(fake.MockDescribeAutoScalingGroupOutput(map[string]fake.ASGtags{
					"asg1": {
						"foo":                          "bar",
						clusterIDTagPrefix + clusterID: resourceLifecycleOwned,
					},
					"asg2": {
						"foo":                          "bar",
						clusterIDTagPrefix + clusterID: resourceLifecycleOwned,
					},
					"asg3": {
						"foo":                          "bar",
						clusterIDTagPrefix + clusterID: resourceLifecycleOwned,
					}}), nil),
			},
			7,
			[]string{"asg1", "asg2", "asg3"},
			[]string{"sgl1"},
			[]string{"sgl1"},
			[]string{"sgl2"},
			false,
		},
		{
			"remove-all-except-first-asg",
			fake.EC2Outputs{DescribeInstances: fake.R(fake.MockDescribeInstancesOutput(
				fake.TestInstance{Id: "foo0", Tags: fake.Tags{"aws:autoscaling:groupName": "asg1"}, PrivateIp: "1.2.3.3", VpcId: "1", State: runningState},
				fake.TestInstance{Id: "foo1", Tags: fake.Tags{"aws:autoscaling:groupName": "asg1"}, PrivateIp: "1.2.3.4", VpcId: "1", State: runningState},
				fake.TestInstance{Id: "foo2", Tags: fake.Tags{"aws:autoscaling:groupName": "asg1"}, PrivateIp: "1.2.3.5", VpcId: "1", State: runningState},
			), nil)},
			fake.ASGOutputs{
				DescribeAutoScalingGroups: fake.R(fake.MockDescribeAutoScalingGroupOutput(map[string]fake.ASGtags{"asg1": {
					"foo":                          "bar",
					clusterIDTagPrefix + clusterID: resourceLifecycleOwned,
				}}), nil),
			},
			3,
			[]string{"asg1"},
			[]string{},
			[]string{},
			[]string{"sgl2", "sgl1"},
			false,
		},
		{
			"add-remove-simultaneously",
			fake.EC2Outputs{DescribeInstances: fake.R(fake.MockDescribeInstancesOutput(
				fake.TestInstance{Id: "foo0", Tags: fake.Tags{"aws:autoscaling:groupName": "asg1"}, PrivateIp: "1.2.3.3", VpcId: "1", State: runningState},
				fake.TestInstance{Id: "sgl1", Tags: fake.Tags{"Name": "node1"}, PrivateIp: "0.1.1.1", VpcId: "1", State: runningState},
			), nil)},
			fake.ASGOutputs{
				DescribeAutoScalingGroups: fake.R(fake.MockDescribeAutoScalingGroupOutput(map[string]fake.ASGtags{"asg1": {
					"foo":                          "bar",
					clusterIDTagPrefix + clusterID: resourceLifecycleOwned,
				}}), nil),
			},
			2,
			[]string{"asg1"},
			[]string{"sgl1"},
			[]string{"sgl1"},
			[]string{"sgl2", "sgl1"},
			false,
		},
	} {
		tt.Run(fmt.Sprintf("%v", test.name), func(t *testing.T) {
			a.ec2 = &fake.EC2Client{Outputs: test.ec2responses}
			a.autoscaling = &fake.ASGClient{Outputs: test.asgresponses}
			err := a.UpdateAutoScalingGroupsAndInstances(context.Background())
			if test.wantError && err == nil {
				t.Errorf("expected error, got nothing")
			}
			if !test.wantError && err != nil {
				t.Errorf("unexpected error '%s'", err)
			}
			if !test.wantError && err == nil {
				adapterSingleIds := a.SingleInstances()
				adapterRunningIds := a.RunningSingleInstances()
				adapterObsoleteIds := a.ObsoleteSingleInstances()
				asgs := make([]string, 0, len(a.TargetedAutoScalingGroups))
				for name := range a.TargetedAutoScalingGroups {
					asgs = append(asgs, name)
				}
				sort.Strings(adapterSingleIds)
				sort.Strings(adapterRunningIds)
				sort.Strings(adapterObsoleteIds)
				sort.Strings(asgs)
				sort.Strings(test.wantSingleInstances)
				sort.Strings(test.wantRunningSingleInstances)
				sort.Strings(test.wantObsoleteInstances)
				sort.Strings(test.wantAsgs)
				if !reflect.DeepEqual(test.wantSingleInstances, adapterSingleIds) {
					t.Errorf("unexpected singleInstances result. wanted %#v, got %#v", test.wantSingleInstances, adapterSingleIds)
				}
				if !reflect.DeepEqual(test.wantRunningSingleInstances, adapterRunningIds) {
					t.Errorf("unexpected runningInstances result. wanted %#v, got %#v", test.wantRunningSingleInstances, adapterRunningIds)
				}
				if !reflect.DeepEqual(test.wantObsoleteInstances, adapterObsoleteIds) {
					t.Errorf("unexpected obsoleteInstances result. wanted %#v, got %#v", test.wantObsoleteInstances, adapterObsoleteIds)
				}
				if !reflect.DeepEqual(test.wantAsgs, asgs) {
					t.Errorf("unexpected autoScalingGroups result. wanted %+v, got %+v", test.wantAsgs, asgs)
				}
				if a.CachedInstances() != test.cacheSize {
					t.Errorf("wrong cache size. wanted %d, got %d", test.cacheSize, a.CachedInstances())
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
			scheme:          string(elbv2Types.LoadBalancerSchemeEnumInternetFacing),
			expectedSubnets: []string{"1", "2"},
		},
		{
			name: "should find two public subnets for public LB regardless of details order",
			subnets: []*subnetDetails{
				{
					availabilityZone: "b",
					public:           true,
					id:               "2",
				},
				{
					availabilityZone: "a",
					public:           true,
					id:               "1",
				},
			},
			scheme:          string(elbv2Types.LoadBalancerSchemeEnumInternetFacing),
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
			scheme:          string(elbv2Types.LoadBalancerSchemeEnumInternetFacing),
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
			scheme:          string(elbv2Types.LoadBalancerSchemeEnumInternetFacing),
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
			scheme:          string(elbv2Types.LoadBalancerSchemeEnumInternetFacing),
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
			scheme:          string(elbv2Types.LoadBalancerSchemeEnumInternal),
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

			for i, subnet := range subnets {
				if subnet != test.expectedSubnets[i] {
					t.Errorf("expected subnet %v, got %v", test.expectedSubnets[i], subnet)
				}
			}
		})
	}
}

func TestParseFilterTagsDefault(t *testing.T) {
	for _, test := range []struct {
		name         string
		clusterID    string
		customFilter string
		want         map[string][]string
	}{
		{
			name:      "success-default-filter-asg",
			clusterID: "mycluster",
			want: map[string][]string{
				clusterIDTagPrefix + "mycluster": {resourceLifecycleOwned},
			},
		},
		{
			name:         "success-custom-filter-asg",
			clusterID:    "aws:12345678910:eu-central-1:mycluster",
			customFilter: "tag:kubernetes.io/cluster/aws:12345678910:eu-central-1:mycluster=owned tag:node.kubernetes.io/role=worker",
			want: map[string][]string{
				clusterIDTagPrefix + "aws:12345678910:eu-central-1:mycluster": {resourceLifecycleOwned},
				"node.kubernetes.io/role":                                     {"worker"},
			},
		},
	} {
		t.Run(fmt.Sprintf("%v", test.name), func(t *testing.T) {
			a := Adapter{
				customFilter: test.customFilter,
				ec2Details:   map[string]*instanceDetails{},
				manifest: &manifest{
					instance: &instanceDetails{
						tags: map[string]string{
							clusterIDTagPrefix + test.clusterID: resourceLifecycleOwned,
						},
					},
				},
			}
			got := a.parseAutoscaleFilterTags(test.clusterID)

			if !reflect.DeepEqual(test.want, got) {
				t.Errorf("unexpected result. wanted %+v, got %+v", test.want, got)
			}
		})
	}
}

func TestParseFilterTagsCustom(t *testing.T) {
	for _, test := range []struct {
		name         string
		clusterId    string
		customFilter string
		want         map[string][]string
	}{
		{
			"success-custom-filter-asg",
			"mycluster",
			"tag:kubernetes.io/cluster/mycluster=owned tag-key=k8s.io/role/node tag-key=custom.com/ingress",
			map[string][]string{
				"kubernetes.io/cluster/mycluster": {"owned"},
				"k8s.io/role/node":                {},
				"custom.com/ingress":              {},
			},
		},
		{
			"success-custom-filter-multivalue-asg",
			"mycluster",
			"tag:kubernetes.io/cluster/mycluster=owned tag-key=k8s.io/role/node tag:custom.com/ingress=owned,shared",
			map[string][]string{
				"kubernetes.io/cluster/mycluster": {"owned"},
				"k8s.io/role/node":                {},
				"custom.com/ingress":              {"owned", "shared"},
			},
		},
		{
			"success-custom-filter-fallback-to-default-asg",
			"mycluster",
			"tag:goodtag=foo tag-key=alsogood thisisabadtag andthisonetoo",
			map[string][]string{
				clusterIDTagPrefix + "mycluster": {resourceLifecycleOwned},
			},
		},
	} {
		t.Run(fmt.Sprintf("%v", test.name), func(t *testing.T) {
			a := Adapter{
				customFilter: test.customFilter,
				ec2Details:   map[string]*instanceDetails{},
				manifest: &manifest{
					instance: &instanceDetails{
						tags: map[string]string{
							clusterIDTagPrefix + test.clusterId: resourceLifecycleOwned,
						},
					},
				},
			}
			got := a.parseAutoscaleFilterTags(test.clusterId)

			if !reflect.DeepEqual(test.want, got) {
				t.Errorf("unexpected result. wanted %+v, got %+v", test.want, got)
			}
		})
	}
}

func TestNonTargetedASGs(t *testing.T) {
	for _, test := range []struct {
		name                    string
		ownedASGs               map[string]*autoScalingGroupDetails
		targetedASGs            map[string]*autoScalingGroupDetails
		expectedNonTargetedASGs map[string]*autoScalingGroupDetails
	}{
		{
			name: "",
			ownedASGs: map[string]*autoScalingGroupDetails{
				"a": nil,
				"b": nil,
				"c": nil,
			},
			targetedASGs: map[string]*autoScalingGroupDetails{
				"b": nil,
				"c": nil,
				"d": nil,
			},
			expectedNonTargetedASGs: map[string]*autoScalingGroupDetails{
				"a": nil,
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			nonTargetedASGs := nonTargetedASGs(test.ownedASGs, test.targetedASGs)
			require.Equal(t, test.expectedNonTargetedASGs, nonTargetedASGs)
		})
	}
}

func TestWithTargetPort(t *testing.T) {
	t.Run("WithTargetPort sets the targetPort property", func(t *testing.T) {
		a := Adapter{}
		port := uint(9977)
		b := a.WithTargetPort(port)
		require.Equal(t, port, b.targetPort)
	})
}

func TestWithTargetHTTPS(t *testing.T) {
	t.Run("WithTargetHTTPS sets the targetHTTPS property", func(t *testing.T) {
		a := Adapter{}
		b := a.WithTargetHTTPS(true)
		require.Equal(t, true, b.targetHTTPS)
	})
}

func TestWithxlbHealthyThresholdCount(t *testing.T) {
	t.Run("WithAlbHealthyThresholdCount sets the albHealthyThresholdCount property", func(t *testing.T) {
		a := Adapter{}
		b := a.WithAlbHealthyThresholdCount(2)
		require.Equal(t, uint(2), b.albHealthyThresholdCount)
	})

	t.Run("WithAlbUnhealthyThresholdCount sets the albUnhealthyThresholdCount property", func(t *testing.T) {
		a := Adapter{}
		b := a.WithAlbUnhealthyThresholdCount(3)
		require.Equal(t, uint(3), b.albUnhealthyThresholdCount)
	})

	t.Run("WithNlbHealthyThresholdCount sets the nlbHealthyThresholdCount property", func(t *testing.T) {
		a := Adapter{}
		b := a.WithNlbHealthyThresholdCount(4)
		require.Equal(t, uint(4), b.nlbHealthyThresholdCount)
	})
}

func TestAdapter_SetTargetsOnCNITargetGroups(t *testing.T) {
	tgARNs := []string{"asg1"}
	thOut := elbv2.DescribeTargetHealthOutput{TargetHealthDescriptions: []elbv2Types.TargetHealthDescription{}}
	m := &fake.ELBv2Client{
		Outputs: fake.ELBv2Outputs{
			DescribeTargetHealth: fake.R(&thOut, nil),
			RegisterTargets:      fake.R(fake.MockDeregisterTargetsOutput(), nil),
			DeregisterTargets:    fake.R(fake.MockDeregisterTargetsOutput(), nil),
		},
	}
	a := &Adapter{elbv2: m, TargetCNI: &TargetCNIconfig{}}

	t.Run("adding a new endpoint", func(t *testing.T) {
		require.NoError(t, a.SetTargetsOnCNITargetGroups(context.Background(), []string{"1.1.1.1"}, tgARNs))
		require.Equal(t, []*elbv2.RegisterTargetsInput{{
			TargetGroupArn: aws.String("asg1"),
			Targets:        []elbv2Types.TargetDescription{{Id: aws.String("1.1.1.1")}},
		}}, m.Rtinputs)
		require.Equal(t, []*elbv2.DeregisterTargetsInput(nil), m.Dtinputs)
	})

	t.Run("two new endpoints, registers the new EPs only", func(t *testing.T) {
		thOut = elbv2.DescribeTargetHealthOutput{TargetHealthDescriptions: []elbv2Types.TargetHealthDescription{
			{Target: &elbv2Types.TargetDescription{Id: aws.String("1.1.1.1")}}},
		}
		m.Rtinputs, m.Dtinputs = nil, nil

		require.NoError(t, a.SetTargetsOnCNITargetGroups(context.Background(), []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"}, tgARNs))
		require.Equal(t, []elbv2Types.TargetDescription{
			{Id: aws.String("2.2.2.2")},
			{Id: aws.String("3.3.3.3")},
		}, m.Rtinputs[0].Targets)
		require.Equal(t, []*elbv2.DeregisterTargetsInput(nil), m.Dtinputs)
	})

	t.Run("removing one endpoint, causing deregistration of it", func(t *testing.T) {
		thOut = elbv2.DescribeTargetHealthOutput{TargetHealthDescriptions: []elbv2Types.TargetHealthDescription{
			{Target: &elbv2Types.TargetDescription{Id: aws.String("1.1.1.1")}},
			{Target: &elbv2Types.TargetDescription{Id: aws.String("2.2.2.2")}},
			{Target: &elbv2Types.TargetDescription{Id: aws.String("3.3.3.3")}},
		}}
		m.Rtinputs, m.Dtinputs = nil, nil

		require.NoError(t, a.SetTargetsOnCNITargetGroups(context.Background(), []string{"1.1.1.1", "3.3.3.3"}, tgARNs))
		require.Equal(t, []*elbv2.RegisterTargetsInput(nil), m.Rtinputs)
		require.Equal(t, []elbv2Types.TargetDescription{{Id: aws.String("2.2.2.2")}}, m.Dtinputs[0].Targets)
	})

	t.Run("restoring desired State after external manipulation, adding and removing one", func(t *testing.T) {
		thOut = elbv2.DescribeTargetHealthOutput{TargetHealthDescriptions: []elbv2Types.TargetHealthDescription{
			{Target: &elbv2Types.TargetDescription{Id: aws.String("1.1.1.1")}},
			{Target: &elbv2Types.TargetDescription{Id: aws.String("2.2.2.2")}},
			{Target: &elbv2Types.TargetDescription{Id: aws.String("4.4.4.4")}},
		}}
		m.Rtinputs, m.Dtinputs = nil, nil

		require.NoError(t, a.SetTargetsOnCNITargetGroups(context.Background(), []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"}, tgARNs))
		require.Equal(t, []elbv2Types.TargetDescription{{Id: aws.String("3.3.3.3")}}, m.Rtinputs[0].Targets)
		require.Equal(t, []elbv2Types.TargetDescription{{Id: aws.String("4.4.4.4")}}, m.Dtinputs[0].Targets)
	})
}

func TestWithTargetAccessMode(t *testing.T) {
	t.Run("WithTargetAccessMode AWSCNI", func(t *testing.T) {
		a := &Adapter{TargetCNI: &TargetCNIconfig{Enabled: false}}
		a = a.WithTargetAccessMode("AWSCNI")

		assert.Equal(t, elbv2Types.TargetTypeEnumIp, a.targetType)
		assert.True(t, a.TargetCNI.Enabled)
	})
	t.Run("WithTargetAccessMode HostPort", func(t *testing.T) {
		a := &Adapter{TargetCNI: &TargetCNIconfig{Enabled: true}}
		a = a.WithTargetAccessMode("HostPort")

		assert.Equal(t, elbv2Types.TargetTypeEnumInstance, a.targetType)
		assert.False(t, a.TargetCNI.Enabled)
	})
	t.Run("WithTargetAccessMode Legacy", func(t *testing.T) {
		a := &Adapter{TargetCNI: &TargetCNIconfig{Enabled: true}}
		a = a.WithTargetAccessMode("Legacy")

		assert.Equal(t, elbv2Types.TargetTypeEnum(""), a.targetType)
		assert.False(t, a.TargetCNI.Enabled)
	})
}

func TestGetStackLBStates(t *testing.T) {
	tests := []struct {
		name                  string
		elbv2Outputs          fake.ELBv2Outputs
		stacks                []*Stack
		expectedStackLBStates []*StackLBState
		expectError           bool
	}{
		{
			name: "success - matching ELBs",
			elbv2Outputs: fake.ELBv2Outputs{
				DescribeLoadBalancers: fake.R(&elbv2.DescribeLoadBalancersOutput{
					LoadBalancers: []elbv2Types.LoadBalancer{
						{
							LoadBalancerArn: aws.String("arn1"),
							State:           &elbv2Types.LoadBalancerState{Code: elbv2Types.LoadBalancerStateEnumActive},
						},
						{
							LoadBalancerArn: aws.String("arn2"),
							State:           &elbv2Types.LoadBalancerState{Code: elbv2Types.LoadBalancerStateEnumFailed},
						},
					},
				}, nil),
			},
			stacks: []*Stack{
				{Name: "stack1", LoadBalancerARN: "arn1"},
				{Name: "stack2", LoadBalancerARN: "arn2"},
			},
			expectedStackLBStates: []*StackLBState{
				{
					Stack:   &Stack{Name: "stack1", LoadBalancerARN: "arn1"},
					LBState: &LoadBalancerState{StateCode: elbv2Types.LoadBalancerStateEnumActive},
				},
				{
					Stack:   &Stack{Name: "stack2", LoadBalancerARN: "arn2"},
					LBState: &LoadBalancerState{StateCode: elbv2Types.LoadBalancerStateEnumFailed},
				},
			},
			expectError: false,
		},
		{
			name: "success - no matching ELB found for one of the stacks",
			elbv2Outputs: fake.ELBv2Outputs{
				DescribeLoadBalancers: fake.R(&elbv2.DescribeLoadBalancersOutput{
					LoadBalancers: []elbv2Types.LoadBalancer{
						{
							LoadBalancerArn: aws.String("arn1"),
							State:           &elbv2Types.LoadBalancerState{Code: elbv2Types.LoadBalancerStateEnumActive},
						},
					},
				}, nil),
			},
			stacks: []*Stack{
				{Name: "stack1", LoadBalancerARN: "arn1"},
				{Name: "stack2", LoadBalancerARN: "arn2"},
			},
			expectedStackLBStates: []*StackLBState{
				{
					Stack:   &Stack{Name: "stack1", LoadBalancerARN: "arn1"},
					LBState: &LoadBalancerState{StateCode: elbv2Types.LoadBalancerStateEnumActive},
				},
				{
					Stack:   &Stack{Name: "stack2", LoadBalancerARN: "arn2"},
					LBState: nil, // No matching ELB found for stack2
				},
			},
			expectError: false,
		},
		{
			name: "error - DescribeLoadBalancers API failure",
			elbv2Outputs: fake.ELBv2Outputs{
				DescribeLoadBalancers: fake.R(nil, fmt.Errorf("API error")),
			},
			stacks:                []*Stack{{Name: "stack1", LoadBalancerARN: "arn1"}},
			expectedStackLBStates: nil,
			expectError:           true,
		},
		{
			name: "success - no LB arn in some stacks",
			elbv2Outputs: fake.ELBv2Outputs{
				DescribeLoadBalancers: fake.R(&elbv2.DescribeLoadBalancersOutput{
					LoadBalancers: []elbv2Types.LoadBalancer{
						{
							LoadBalancerArn: aws.String("arn1"),
							State:           &elbv2Types.LoadBalancerState{Code: elbv2Types.LoadBalancerStateEnumActive},
						},
					},
				}, nil),
			},
			stacks: []*Stack{
				{Name: "stack1", LoadBalancerARN: "arn1"},
				{Name: "stack2"},
			},
			expectedStackLBStates: []*StackLBState{
				{
					Stack:   &Stack{Name: "stack1", LoadBalancerARN: "arn1"},
					LBState: &LoadBalancerState{StateCode: elbv2Types.LoadBalancerStateEnumActive},
				},
				{
					Stack:   &Stack{Name: "stack2"},
					LBState: nil, // No LoadBalancerARN provided for stack2
				},
			},
			expectError: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			a := &Adapter{
				elbv2: &fake.ELBv2Client{
					Outputs: test.elbv2Outputs,
				},
			}

			stackLBStates, err := a.GetStackLBStates(context.Background(), test.stacks)

			if test.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Len(t, stackLBStates, len(test.expectedStackLBStates))
				for i, expected := range test.expectedStackLBStates {
					require.Equal(t, expected.Stack, stackLBStates[i].Stack)
					if expected.LBState == nil {
						require.Nil(t, stackLBStates[i].LBState)
					} else {
						require.Equal(t, expected.LBState.StateCode, stackLBStates[i].LBState.StateCode)
					}
				}
			}
		})
	}
}
