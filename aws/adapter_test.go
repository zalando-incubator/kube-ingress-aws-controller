package aws

import (
	"sort"
	"testing"

	"github.com/aws/aws-sdk-go/service/elbv2"
)

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
