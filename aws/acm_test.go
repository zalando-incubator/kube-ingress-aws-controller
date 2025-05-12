package aws

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	"github.com/aws/aws-sdk-go-v2/service/acm/types"
	"github.com/stretchr/testify/require"
	"github.com/zalando-incubator/kube-ingress-aws-controller/aws/fake"
)

type acmExpect struct {
	ARN         string
	DomainNames []string
	Chain       int
	Error       string
	EmptyList   bool
}

func TestACM(t *testing.T) {
	cert := mustRead("acm.txt")
	chain := mustRead("chain.txt")

	for _, ti := range []struct {
		msg       string
		api       ACMAPI
		filterTag string
		expect    acmExpect
	}{
		{
			msg: "Found ACM Cert foobar and a chain",
			api: fake.NewACMClient(
				map[string]*acm.GetCertificateOutput{
					"foobar": {
						Certificate:      aws.String(cert),
						CertificateChain: aws.String(chain),
					},
				},
				nil, nil,
			),
			expect: acmExpect{
				ARN:         "foobar",
				DomainNames: []string{"foobar.de"},
			},
		},
		{
			msg: "Found ACM Cert foobar and no chain",
			api: fake.NewACMClient(
				map[string]*acm.GetCertificateOutput{
					"foobar": {
						Certificate: aws.String(cert),
					},
				},
				nil, nil,
			),
			expect: acmExpect{
				ARN:         "foobar",
				DomainNames: []string{"foobar.de"},
			},
		},
		{
			msg: "Found one ACM Cert with correct filter tag",
			api: fake.NewACMClient(
				map[string]*acm.GetCertificateOutput{
					"foobar": {
						Certificate: aws.String(cert),
					},
					"foobaz": {
						Certificate: aws.String(cert),
					},
				},
				map[string]*acm.ListTagsForCertificateOutput{
					"foobar": {
						Tags: []types.Tag{{Key: aws.String("production"), Value: aws.String("true")}},
					},
					"foobaz": {
						Tags: []types.Tag{{Key: aws.String("production"), Value: aws.String("false")}},
					},
				},
				nil,
			),
			filterTag: "production=true",
			expect: acmExpect{
				ARN:         "foobar",
				DomainNames: []string{"foobar.de"},
			},
		},
		{
			msg: "ACM Cert with incorrect filter tag should not be found",
			api: fake.NewACMClient(
				map[string]*acm.GetCertificateOutput{
					"foobar": {
						Certificate: aws.String(cert),
					},
				},
				map[string]*acm.ListTagsForCertificateOutput{
					"foobar": {
						Tags: []types.Tag{{Key: aws.String("production"), Value: aws.String("false")}},
					},
				},
				nil,
			),
			filterTag: "production=true",
			expect: acmExpect{
				EmptyList:   true,
				ARN:         "foobar",
				DomainNames: []string{"foobar.de"},
			},
		},
		{
			msg: "Fail on ListCertificates error",
			api: fake.NewACMClient(
				nil, nil,
				map[string]error{
					"ListCertificates": fmt.Errorf("ListCertificates error"),
				},
			),
			filterTag: "production=true",
			expect: acmExpect{
				Error: "failed to list certificates page: ListCertificates error",
			},
		},
	} {
		t.Run(ti.msg, func(t *testing.T) {
			provider := newACMCertProvider(ti.api, ti.filterTag)
			list, err := provider.GetCertificates(context.Background())

			if ti.expect.Error != "" {
				require.EqualError(t, err, ti.expect.Error)
				return
			} else {
				require.NoError(t, err)
			}

			if ti.expect.EmptyList {
				require.Equal(t, 0, len(list))
			} else {
				require.Equal(t, 1, len(list))

				cert := list[0]
				require.Equal(t, ti.expect.ARN, cert.ID())
				require.Equal(t, ti.expect.DomainNames, cert.DomainNames())
			}
		})
	}
}
