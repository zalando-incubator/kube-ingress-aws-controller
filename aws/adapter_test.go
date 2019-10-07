package aws

import (
	"fmt"
	"os"
	"reflect"
	"sort"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/elbv2"
)

func TesGenerateDefaultFilters(tt *testing.T) {
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
			if aws.StringValue(filters[0].Name) != "tag:"+clusterIDTagPrefix+test.clusterId {
				t.Errorf("generateDefaultFilters first filter has wrong name %s", aws.StringValue(filters[0].Name))
			}
			if len(filters[0].Values) != 1 {
				t.Errorf("generateDefaultFilters first filter has %d values instead of 1", len(filters[0].Values))
			}
			if aws.StringValue(filters[0].Values[0]) != resourceLifecycleOwned {
				t.Errorf("generateDefaultFilters first filter has wrong value %s", aws.StringValue(filters[0].Values[0]))
			}
			if aws.StringValue(filters[1].Name) != "tag-key" {
				t.Errorf("generateDefaultFilters second filter has wrong name %s", aws.StringValue(filters[1].Name))
			}
			if len(filters[1].Values) != 1 {
				t.Errorf("generateDefaultFilters second filter has %d values instead of 1", len(filters[1].Values))
			}
			if aws.StringValue(filters[1].Values[0]) != kubernetesNodeRoleTag {
				t.Errorf("generateDefaultFilters second filter has wrong value %s", aws.StringValue(filters[1].Values[0]))
			}
		})
	}
}

func TestParseFilters(tt *testing.T) {
	for _, test := range []struct {
		name            string
		customFilter    *string
		clusterId       string
		expectedFilters []*ec2.Filter
	}{
		{
			"no-custom-filter",
			nil,
			"cluster",
			[]*ec2.Filter{
				{
					Name:   aws.String("tag:" + clusterIDTagPrefix + "cluster"),
					Values: aws.StringSlice([]string{resourceLifecycleOwned}),
				},
				{
					Name:   aws.String("tag-key"),
					Values: aws.StringSlice([]string{kubernetesNodeRoleTag}),
				},
			},
		},
		{
			"custom-filter1",
			aws.String("tag:Test=test"),
			"cluster",
			[]*ec2.Filter{
				{
					Name:   aws.String("tag:Test"),
					Values: aws.StringSlice([]string{"test"}),
				},
			},
		},
		{
			"custom-filter2",
			aws.String("tag:Test=test vpc-id=id1,id2"),
			"cluster",
			[]*ec2.Filter{
				{
					Name:   aws.String("tag:Test"),
					Values: aws.StringSlice([]string{"test"}),
				},
				{
					Name:   aws.String("vpc-id"),
					Values: aws.StringSlice([]string{"id1", "id2"}),
				},
			},
		},
		{
			"custom-filter3",
			aws.String("tag:Test=test tag:Test=test1,test2  tag-key=key1,key2,key3"),
			"cluster",
			[]*ec2.Filter{
				{
					Name:   aws.String("tag:Test"),
					Values: aws.StringSlice([]string{"test"}),
				},
				{
					Name:   aws.String("tag:Test"),
					Values: aws.StringSlice([]string{"test1", "test2"}),
				},
				{
					Name:   aws.String("tag-key"),
					Values: aws.StringSlice([]string{"key1", "key2", "key3"}),
				},
			},
		},
		{
			"illegal1",
			aws.String("test"),
			"cluster",
			[]*ec2.Filter{
				{
					Name:   aws.String("tag:" + clusterIDTagPrefix + "cluster"),
					Values: aws.StringSlice([]string{resourceLifecycleOwned}),
				},
				{
					Name:   aws.String("tag-key"),
					Values: aws.StringSlice([]string{kubernetesNodeRoleTag}),
				},
			},
		},
	} {
		tt.Run(fmt.Sprintf("%v", test.name), func(t *testing.T) {
			tt.Log(test.name)
			if test.customFilter != nil {
				os.Setenv(customTagFilterEnvVarName, *test.customFilter)
			} else {
				os.Unsetenv(customTagFilterEnvVarName)
			}
			output := parseFilters(test.clusterId)
			if !reflect.DeepEqual(test.expectedFilters, output) {
				t.Errorf("unexpected result. wanted %q, got %q", test.expectedFilters, output)
			}
		})
	}
}

func TestFiltersString(tt *testing.T) {
	for _, test := range []struct {
		name    string
		filters []*ec2.Filter
		str     string
	}{
		{
			"test1",
			[]*ec2.Filter{
				{
					Name:   aws.String("tag:" + clusterIDTagPrefix + "cluster"),
					Values: aws.StringSlice([]string{resourceLifecycleOwned}),
				},
				{
					Name:   aws.String("tag-key"),
					Values: aws.StringSlice([]string{kubernetesNodeRoleTag}),
				},
			},
			"tag:" + clusterIDTagPrefix + "cluster=" + resourceLifecycleOwned + " tag-key=" + kubernetesNodeRoleTag,
		},
		{
			"test2",
			[]*ec2.Filter{
				{
					Name:   aws.String("tag:Test"),
					Values: aws.StringSlice([]string{"test"}),
				},
			},
			"tag:Test=test",
		},
		{
			"custom-filter2",
			[]*ec2.Filter{
				{
					Name:   aws.String("tag:Test"),
					Values: aws.StringSlice([]string{"test"}),
				},
				{
					Name:   aws.String("vpc-id"),
					Values: aws.StringSlice([]string{"id1", "id2"}),
				},
			},
			"tag:Test=test vpc-id=id1,id2",
		},
		{
			"custom-filter3",
			[]*ec2.Filter{
				{
					Name:   aws.String("tag:Test"),
					Values: aws.StringSlice([]string{"test"}),
				},
				{
					Name:   aws.String("tag:Test"),
					Values: aws.StringSlice([]string{"test1", "test2"}),
				},
				{
					Name:   aws.String("tag-key"),
					Values: aws.StringSlice([]string{"key1", "key2", "key3"}),
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
		autoScalingGroups: make(map[string]*autoScalingGroupDetails),
		singleInstances:   make(map[string]*instanceDetails),
		obsoleteInstances: make([]string, 0),
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
		ec2responses               ec2MockOutputs
		asgresponses               autoscalingMockOutputs
		cacheSize                  int
		wantAsgs                   []string
		wantSingleInstances        []string
		wantRunningSingleInstances []string
		wantObsoleteInstances      []string
		wantError                  bool
	}{
		{
			"initial",
			ec2MockOutputs{describeInstancesPages: mockDIPOutput(
				nil,
				testInstance{id: "foo0", tags: tags{"aws:autoscaling:groupName": "asg1"}, privateIp: "1.2.3.3", vpcId: "1", state: runningState},
				testInstance{id: "foo1", tags: tags{"aws:autoscaling:groupName": "asg1"}, privateIp: "1.2.3.4", vpcId: "1", state: runningState},
			)},
			autoscalingMockOutputs{
				describeAutoScalingGroups: R(mockDASGOutput(map[string]asgtags{"asg1": {
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
			ec2MockOutputs{describeInstancesPages: mockDIPOutput(
				nil,
				testInstance{id: "foo0", tags: tags{"aws:autoscaling:groupName": "asg1"}, privateIp: "1.2.3.3", vpcId: "1", state: runningState},
				testInstance{id: "foo1", tags: tags{"aws:autoscaling:groupName": "asg1"}, privateIp: "1.2.3.4", vpcId: "1", state: 0},
				testInstance{id: "foo2", tags: tags{"aws:autoscaling:groupName": "asg1"}, privateIp: "1.2.3.5", vpcId: "1", state: runningState},
			)},
			autoscalingMockOutputs{
				describeAutoScalingGroups: R(mockDASGOutput(map[string]asgtags{"asg1": {
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
			ec2MockOutputs{describeInstancesPages: mockDIPOutput(
				nil,
				testInstance{id: "foo0", tags: tags{"aws:autoscaling:groupName": "asg1"}, privateIp: "1.2.3.3", vpcId: "1", state: runningState},
				testInstance{id: "foo1", tags: tags{"aws:autoscaling:groupName": "asg1"}, privateIp: "1.2.3.4", vpcId: "1", state: runningState},
				testInstance{id: "foo2", tags: tags{"aws:autoscaling:groupName": "asg1"}, privateIp: "1.2.3.5", vpcId: "1", state: runningState},
				testInstance{id: "bar1", tags: tags{"aws:autoscaling:groupName": "asg2"}, privateIp: "2.1.1.1", vpcId: "1", state: runningState},
			)},
			autoscalingMockOutputs{
				describeAutoScalingGroups: R(mockDASGOutput(map[string]asgtags{
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
			ec2MockOutputs{describeInstancesPages: mockDIPOutput(
				nil,
				testInstance{id: "foo0", tags: tags{"aws:autoscaling:groupName": "asg1"}, privateIp: "1.2.3.3", vpcId: "1", state: runningState},
				testInstance{id: "foo1", tags: tags{"aws:autoscaling:groupName": "asg1"}, privateIp: "1.2.3.4", vpcId: "1", state: runningState},
				testInstance{id: "foo2", tags: tags{"aws:autoscaling:groupName": "asg1"}, privateIp: "1.2.3.5", vpcId: "1", state: runningState},
				testInstance{id: "bar1", tags: tags{"aws:autoscaling:groupName": "asg2"}, privateIp: "2.1.1.1", vpcId: "1", state: runningState},
				testInstance{id: "bar2", tags: tags{"aws:autoscaling:groupName": "asg2"}, privateIp: "2.2.2.2", vpcId: "1", state: runningState},
			)},
			autoscalingMockOutputs{
				describeAutoScalingGroups: R(mockDASGOutput(map[string]asgtags{
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
			ec2MockOutputs{describeInstancesPages: mockDIPOutput(
				nil,
				testInstance{id: "foo0", tags: tags{"aws:autoscaling:groupName": "asg1"}, privateIp: "1.2.3.3", vpcId: "1", state: runningState},
				testInstance{id: "foo1", tags: tags{"aws:autoscaling:groupName": "asg1"}, privateIp: "1.2.3.4", vpcId: "1", state: runningState},
				testInstance{id: "foo2", tags: tags{"aws:autoscaling:groupName": "asg1"}, privateIp: "1.2.3.5", vpcId: "1", state: runningState},
				testInstance{id: "bar1", tags: tags{"aws:autoscaling:groupName": "asg2"}, privateIp: "2.1.1.1", vpcId: "1", state: runningState},
				testInstance{id: "bar2", tags: tags{"aws:autoscaling:groupName": "asg2"}, privateIp: "2.2.2.2", vpcId: "1", state: runningState},
				testInstance{id: "baz1", tags: tags{"aws:autoscaling:groupName": "asg3"}, privateIp: "3.1.1.1", vpcId: "1", state: runningState},
			)},
			autoscalingMockOutputs{
				describeAutoScalingGroups: R(mockDASGOutput(map[string]asgtags{
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
			ec2MockOutputs{describeInstancesPages: mockDIPOutput(
				nil,
				testInstance{id: "foo0", tags: tags{"aws:autoscaling:groupName": "asg1"}, privateIp: "1.2.3.3", vpcId: "1", state: runningState},
				testInstance{id: "foo1", tags: tags{"aws:autoscaling:groupName": "asg1"}, privateIp: "1.2.3.4", vpcId: "1", state: runningState},
				testInstance{id: "foo2", tags: tags{"aws:autoscaling:groupName": "asg1"}, privateIp: "1.2.3.5", vpcId: "1", state: runningState},
				testInstance{id: "bar1", tags: tags{"aws:autoscaling:groupName": "asg2"}, privateIp: "2.1.1.1", vpcId: "1", state: runningState},
				testInstance{id: "bar2", tags: tags{"aws:autoscaling:groupName": "asg2"}, privateIp: "2.2.2.2", vpcId: "1", state: runningState},
				testInstance{id: "baz1", tags: tags{"aws:autoscaling:groupName": "asg3"}, privateIp: "3.1.1.1", vpcId: "1", state: runningState},
				testInstance{id: "sgl1", tags: tags{"Name": "node1"}, privateIp: "0.1.1.1", vpcId: "1", state: runningState},
			)},
			autoscalingMockOutputs{
				describeAutoScalingGroups: R(mockDASGOutput(map[string]asgtags{
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
			ec2MockOutputs{describeInstancesPages: mockDIPOutput(
				nil,
				testInstance{id: "foo0", tags: tags{"aws:autoscaling:groupName": "asg1"}, privateIp: "1.2.3.3", vpcId: "1", state: runningState},
				testInstance{id: "foo1", tags: tags{"aws:autoscaling:groupName": "asg1"}, privateIp: "1.2.3.4", vpcId: "1", state: runningState},
				testInstance{id: "foo2", tags: tags{"aws:autoscaling:groupName": "asg1"}, privateIp: "1.2.3.5", vpcId: "1", state: runningState},
				testInstance{id: "bar1", tags: tags{"aws:autoscaling:groupName": "asg2"}, privateIp: "2.1.1.1", vpcId: "1", state: runningState},
				testInstance{id: "bar2", tags: tags{"aws:autoscaling:groupName": "asg2"}, privateIp: "2.2.2.2", vpcId: "1", state: runningState},
				testInstance{id: "baz1", tags: tags{"aws:autoscaling:groupName": "asg3"}, privateIp: "3.1.1.1", vpcId: "1", state: runningState},
				testInstance{id: "sgl1", tags: tags{"Name": "node1"}, privateIp: "0.1.1.1", vpcId: "1", state: runningState},
				testInstance{id: "sgl2", tags: tags{"Name": "node2"}, privateIp: "0.1.1.2", vpcId: "1", state: stoppedState},
			)},
			autoscalingMockOutputs{
				describeAutoScalingGroups: R(mockDASGOutput(map[string]asgtags{
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
			ec2MockOutputs{describeInstancesPages: mockDIPOutput(
				nil,
				testInstance{id: "foo0", tags: tags{"aws:autoscaling:groupName": "asg1"}, privateIp: "1.2.3.3", vpcId: "1", state: runningState},
				testInstance{id: "foo1", tags: tags{"aws:autoscaling:groupName": "asg1"}, privateIp: "1.2.3.4", vpcId: "1", state: runningState},
				testInstance{id: "foo2", tags: tags{"aws:autoscaling:groupName": "asg1"}, privateIp: "1.2.3.5", vpcId: "1", state: runningState},
				testInstance{id: "bar1", tags: tags{"aws:autoscaling:groupName": "asg2"}, privateIp: "2.1.1.1", vpcId: "1", state: runningState},
				testInstance{id: "bar2", tags: tags{"aws:autoscaling:groupName": "asg2"}, privateIp: "2.2.2.2", vpcId: "1", state: runningState},
				testInstance{id: "sgl1", tags: tags{"Name": "node1"}, privateIp: "0.1.1.1", vpcId: "1", state: runningState},
			)},
			autoscalingMockOutputs{
				describeAutoScalingGroups: R(mockDASGOutput(map[string]asgtags{
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
			ec2MockOutputs{describeInstancesPages: mockDIPOutput(
				errDummy,
				testInstance{},
			)},
			autoscalingMockOutputs{},
			6,
			[]string{"asg1", "asg2"},
			[]string{"sgl1"},
			[]string{"sgl1"},
			[]string{"sgl2"},
			true,
		},
		{
			"error-fetching-asg",
			ec2MockOutputs{describeInstancesPages: mockDIPOutput(
				nil,
				testInstance{id: "foo0", tags: tags{"aws:autoscaling:groupName": "asg1"}, privateIp: "1.2.3.3", vpcId: "1", state: runningState},
				testInstance{id: "foo1", tags: tags{"aws:autoscaling:groupName": "asg1"}, privateIp: "1.2.3.4", vpcId: "1", state: runningState},
				testInstance{id: "foo2", tags: tags{"aws:autoscaling:groupName": "asg1"}, privateIp: "1.2.3.5", vpcId: "1", state: runningState},
				testInstance{id: "bar1", tags: tags{"aws:autoscaling:groupName": "asg2"}, privateIp: "2.1.1.1", vpcId: "1", state: runningState},
				testInstance{id: "bar2", tags: tags{"aws:autoscaling:groupName": "asg2"}, privateIp: "2.2.2.2", vpcId: "1", state: runningState},
				testInstance{id: "sgl1", tags: tags{"Name": "node1"}, privateIp: "0.1.1.1", vpcId: "1", state: runningState},
				testInstance{id: "fail", tags: tags{"aws:autoscaling:groupName": "none"}, privateIp: "0.2.2.2", vpcId: "1", state: runningState},
			)},
			autoscalingMockOutputs{
				describeAutoScalingGroups: R(mockDASGOutput(map[string]asgtags{
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
			ec2MockOutputs{describeInstancesPages: mockDIPOutput(
				nil,
				testInstance{id: "foo0", tags: tags{"aws:autoscaling:groupName": "asg1"}, privateIp: "1.2.3.3", vpcId: "1", state: runningState},
				testInstance{id: "foo1", tags: tags{"aws:autoscaling:groupName": "asg1"}, privateIp: "1.2.3.4", vpcId: "1", state: runningState},
				testInstance{id: "foo2", tags: tags{"aws:autoscaling:groupName": "asg1"}, privateIp: "1.2.3.5", vpcId: "1", state: runningState},
				testInstance{id: "bar1", tags: tags{"aws:autoscaling:groupName": "asg2"}, privateIp: "2.1.1.1", vpcId: "1", state: runningState},
				testInstance{id: "bar2", tags: tags{"aws:autoscaling:groupName": "asg2"}, privateIp: "2.2.2.2", vpcId: "1", state: runningState},
				testInstance{id: "baz1", tags: tags{"aws:autoscaling:groupName": "asg3"}, privateIp: "3.1.1.1", vpcId: "1", state: runningState},
				testInstance{id: "sgl1", tags: tags{"Name": "node1"}, privateIp: "0.1.1.1", vpcId: "1", state: runningState},
			)},
			autoscalingMockOutputs{
				describeAutoScalingGroups: R(mockDASGOutput(map[string]asgtags{
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
			ec2MockOutputs{describeInstancesPages: mockDIPOutput(
				nil,
				testInstance{id: "foo0", tags: tags{"aws:autoscaling:groupName": "asg1"}, privateIp: "1.2.3.3", vpcId: "1", state: runningState},
				testInstance{id: "foo1", tags: tags{"aws:autoscaling:groupName": "asg1"}, privateIp: "1.2.3.4", vpcId: "1", state: runningState},
				testInstance{id: "foo2", tags: tags{"aws:autoscaling:groupName": "asg1"}, privateIp: "1.2.3.5", vpcId: "1", state: runningState},
			)},
			autoscalingMockOutputs{
				describeAutoScalingGroups: R(mockDASGOutput(map[string]asgtags{"asg1": {
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
			ec2MockOutputs{describeInstancesPages: mockDIPOutput(
				nil,
				testInstance{id: "foo0", tags: tags{"aws:autoscaling:groupName": "asg1"}, privateIp: "1.2.3.3", vpcId: "1", state: runningState},
				testInstance{id: "sgl1", tags: tags{"Name": "node1"}, privateIp: "0.1.1.1", vpcId: "1", state: runningState},
			)},
			autoscalingMockOutputs{
				describeAutoScalingGroups: R(mockDASGOutput(map[string]asgtags{"asg1": {
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
			a.ec2 = &mockEc2Client{outputs: test.ec2responses}
			a.autoscaling = &mockAutoScalingClient{outputs: test.asgresponses}
			err := a.UpdateAutoScalingGroupsAndInstances()
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
				asgs := a.AutoScalingGroupNames()
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

func TestParseFilterTagsDefault(t *testing.T) {
        for _, test := range []struct {
                name          string
                clusterId     string
                want          map[string][]string
        }{
                {
                        "success-default-filter-asg",
                        "mycluster",
                        map[string][]string{
                                clusterIDTagPrefix + "mycluster": []string{resourceLifecycleOwned},
                        },
                },
        } {
                t.Run(fmt.Sprintf("%v", test.name), func(t *testing.T) {
                        got := parseAutoscaleFilterTags(test.clusterId)

                        if !reflect.DeepEqual(test.want, got) {
                                t.Errorf("unexpected result. wanted %+v, got %+v", test.want, got)
                        }
                })
        }
}

func TestParseFilterTagsCustom(t *testing.T) {
        for _, test := range []struct {
                name          string
                clusterId     string
                customFilter  string
                want          map[string][]string
        }{
                {
                        "success-custom-filter-asg",
                        "mycluster",
                        "tag:kubernetes.io/cluster/mycluster=owned tag-key=k8s.io/role/node tag-key=custom.com/ingress",
                        map[string][]string{
                                "kubernetes.io/cluster/mycluster": []string{"owned"},
                                "k8s.io/role/node": []string{},
                                "custom.com/ingress": []string{},
                        },
                },
                {
                        "success-custom-filter-multivalue-asg",
                        "mycluster",
                        "tag:kubernetes.io/cluster/mycluster=owned tag-key=k8s.io/role/node tag:custom.com/ingress=owned,shared",
                        map[string][]string{
                                "kubernetes.io/cluster/mycluster": []string{"owned"},
                                "k8s.io/role/node": []string{},
                                "custom.com/ingress": []string{"owned", "shared"},
                        },
                },
                {
                        "success-custom-filter-fallback-to-default-asg",
                        "mycluster",
                        "tag:goodtag=foo tag-key=alsogood thisisabadtag andthisonetoo",
                        map[string][]string{
                                clusterIDTagPrefix + "mycluster": []string{resourceLifecycleOwned},
                        },
                },
        } {
                t.Run(fmt.Sprintf("%v", test.name), func(t *testing.T) {
                        os.Setenv("CUSTOM_FILTERS", test.customFilter)
                        got := parseAutoscaleFilterTags(test.clusterId)
                        os.Unsetenv("CUSTOM_FILTERS")

                        if !reflect.DeepEqual(test.want, got) {
                                t.Errorf("unexpected result. wanted %+v, got %+v", test.want, got)
                        }
                })
        }
}

