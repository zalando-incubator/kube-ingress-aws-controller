package aws

import (
	"fmt"
	"testing"
)

var testNamers = map[string]func(string, []string, string) string{
	"stack": normalizeStackName,
}

func TestNormalization(t *testing.T) {
	for _, test := range []struct {
		clusterID string
		arn       []string
		scheme    string
		want      map[string]string
	}{
		{
			"playground",
			[]string{"arn:aws:acm:eu-central-1:123456789012:certificate/f4bd7ed6-bf23-11e6-8db1-ef7ba1500c61"},
			"internet-facing",
			map[string]string{"": "playground-7cf27d0-internet-facing"},
		},
		{"foo", []string{""}, "internet-facing", map[string]string{"": "foo-fd80709-internet-facing"}},
		{"foo", []string{"bar"}, "internet-facing", map[string]string{"": "foo-1f01f4d-internet-facing"}},
		{"foo", []string{"fake-cert-arn"}, "internet-facing", map[string]string{"": "foo-f398861-internet-facing"}},
		{"test/with-invalid+chars", []string{"bar"}, "internet-facing", map[string]string{"": "test-with-invalid-chars-1f01f4d-internet-facing"}},
		{"-apparently-valid-name", []string{"bar"}, "internet-facing", map[string]string{"": "apparently-valid-name-1f01f4d-internet-facing"}},
		{"apparently-valid-name-", []string{"bar"}, "internet-facing", map[string]string{"": "apparently-valid-name-1f01f4d-internet-facing"}},
		{"valid--name", []string{"bar"}, "internet-facing", map[string]string{"": "valid-name-1f01f4d-internet-facing"}},
		{"valid-----name", []string{"bar"}, "internet-facing", map[string]string{"": "valid-name-1f01f4d-internet-facing"}},
		{"foo bar baz zbr", []string{"bar"}, "internet-facing", map[string]string{"": "foo-bar-baz-zbr-1f01f4d-internet-facing"}},
		{"very long cluster where lb but not stack id needs to be truncated", []string{"bar"}, "internet-facing", map[string]string{
			"stack": "very-long-cluster-where-lb-but-not-stack-id-needs-to-be-truncated-1f01f4d-internet-facing",
		}},
		{"-no---need---to---truncate-", []string{""}, "internet-facing", map[string]string{"": "no-need-to-truncate-fd80709-internet-facing"}},
		{
			"aws:170858875137:eu-central-1:kube-aws-test-ingctl001",
			[]string{"arn:aws:acm:eu-central-1:123456789012:certificate/f4bd7ed6-bf23-11e6-8db1-ef7ba1500c61"},
			"internet-facing",
			map[string]string{
				"stack": "aws-170858875137-eu-central-1-kube-aws-test-ingctl001-7cf27d0-internet-facing",
			},
		},
		{
			"some very long cluster id tha is supposed to test the limits of the stack namer maximum length and truncate this at some point",
			[]string{"bar"},
			"internet-facing",
			map[string]string{
				"stack": "ery-long-cluster-id-tha-is-supposed-to-test-the-limits-of-the-stack-namer-maximum-length-and-truncate-this-at-some-point-1f01f4d-internet-facing",
			},
		},
	} {
		for namerKey, namerFunc := range testNamers {
			t.Run(fmt.Sprintf("%s/%s", namerKey, test.want), func(t *testing.T) {
				got := namerFunc(test.clusterID, test.arn, test.scheme)
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
