package aws

import (
	"fmt"
	"testing"
)

var testNamers = map[string]func(string, string) string{
	"stack": normalizeStackName,
	"elb":   normalizeLoadBalancerName,
}

func TestNormalization(t *testing.T) {
	for _, test := range []struct {
		clusterID string
		arn       string
		want      map[string]string
	}{
		{
			"playground",
			"arn:aws:acm:eu-central-1:123456789012:certificate/f4bd7ed6-bf23-11e6-8db1-ef7ba1500c61",
			map[string]string{"": "playground-7cf27d0"},
		},
		{"foo", "", map[string]string{"": "foo-fd80709"}},
		{"foo", "bar", map[string]string{"": "foo-1f01f4d"}},
		{"foo", "fake-cert-arn", map[string]string{"": "foo-f398861"}},
		{"test/with-invalid+chars", "bar", map[string]string{"": "test-with-invalid-chars-1f01f4d"}},
		{"-apparently-valid-name", "bar", map[string]string{"": "apparently-valid-name-1f01f4d"}},
		{"apparently-valid-name-", "bar", map[string]string{"": "apparently-valid-name-1f01f4d"}},
		{"valid--name", "bar", map[string]string{"": "valid-name-1f01f4d"}},
		{"valid-----name", "bar", map[string]string{"": "valid-name-1f01f4d"}},
		{"foo bar baz zbr", "bar", map[string]string{"": "foo-bar-baz-zbr-1f01f4d"}},
		{"very long cluster where lb but not stack id needs to be truncated", "bar", map[string]string{
			"elb":   "id-needs-to-be-truncated-1f01f4d",
			"stack": "very-long-cluster-where-lb-but-not-stack-id-needs-to-be-truncated-1f01f4d",
		}},
		{"-no---need---to---truncate-", "", map[string]string{"": "no-need-to-truncate-fd80709"}},
		{
			"aws:170858875137:eu-central-1:kube-aws-test-ingctl001",
			"arn:aws:acm:eu-central-1:123456789012:certificate/f4bd7ed6-bf23-11e6-8db1-ef7ba1500c61",
			map[string]string{
				"elb":   "kube-aws-test-ingctl001-7cf27d0",
				"stack": "aws-170858875137-eu-central-1-kube-aws-test-ingctl001-7cf27d0",
			},
		},
		{
			"some very long cluster id tha is supposed to test the limits of the stack namer maximum length and truncate this at some point",
			"bar",
			map[string]string{
				"elb":   "ncate-this-at-some-point-1f01f4d",
				"stack": "ery-long-cluster-id-tha-is-supposed-to-test-the-limits-of-the-stack-namer-maximum-length-and-truncate-this-at-some-point-1f01f4d",
			},
		},
	} {
		for namerKey, namerFunc := range testNamers {
			t.Run(fmt.Sprintf("%s/%s", namerKey, test.want), func(t *testing.T) {
				got := namerFunc(test.clusterID, test.arn)
				var want string
				want, has := test.want[namerKey]
				if !has {
					want = test.want[""]
				}
				if want != got {
					t.Errorf("unexpected normalized name for namer %q. wanted %q, got %q", namerKey, want, got)
				}
			})
		}
	}

}
