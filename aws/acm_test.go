package aws

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/acm"
	"github.com/aws/aws-sdk-go/service/acm/acmiface"
	"github.com/stretchr/testify/require"
	"github.com/zalando-incubator/kube-ingress-aws-controller/aws/fake"
)

type acmExpect struct {
	ARN         string
	DomainNames []string
	Chain       int
	Error       error
	EmptyList   bool
}

func TestACM(t *testing.T) {
	cert := mustRead("acm.txt")
	chain := mustRead("chain.txt")

	for _, ti := range []struct {
		msg       string
		api       acmiface.ACMAPI
		filterTag string
		expect    acmExpect
	}{
		{
			msg: "Found ACM Cert foobar and a chain",
			api: fake.NewACMClient(
				acm.ListCertificatesOutput{
					CertificateSummaryList: []*acm.CertificateSummary{
						{
							CertificateArn: aws.String("foobar"),
							DomainName:     aws.String("foobar.de"),
						},
					},
				},
				acm.GetCertificateOutput{
					Certificate:      aws.String(cert),
					CertificateChain: aws.String(chain),
				},
			),
			expect: acmExpect{
				ARN:         "foobar",
				DomainNames: []string{"foobar.de"},
				Error:       nil,
			},
		},
		{
			msg: "Found ACM Cert foobar and no chain",
			api: fake.NewACMClient(
				acm.ListCertificatesOutput{
					CertificateSummaryList: []*acm.CertificateSummary{
						{
							CertificateArn: aws.String("foobar"),
							DomainName:     aws.String("foobar.de"),
						},
					},
				},
				acm.GetCertificateOutput{
					Certificate: aws.String(cert),
				},
			),
			expect: acmExpect{
				ARN:         "foobar",
				DomainNames: []string{"foobar.de"},
				Error:       nil,
			},
		},
		{
			msg: "Found ACM Cert with correct filter tag",
			api: fake.NewACMClientWithTags(
				acm.ListCertificatesOutput{
					CertificateSummaryList: []*acm.CertificateSummary{
						{
							CertificateArn: aws.String("foobar"),
							DomainName:     aws.String("foobar.de"),
						},
					},
				},
				acm.GetCertificateOutput{
					Certificate: aws.String(cert),
				},
				map[string]*acm.ListTagsForCertificateOutput{
					"foobar": {
						Tags: []*acm.Tag{{Key: aws.String("production"), Value: aws.String("true")}},
					},
				},
			),
			filterTag: "production=true",
			expect: acmExpect{
				ARN:         "foobar",
				DomainNames: []string{"foobar.de"},
				Error:       nil,
			},
		},
		{
			msg: "Found ACM Cert with incorrect filter tag",
			api: fake.NewACMClientWithTags(
				acm.ListCertificatesOutput{
					CertificateSummaryList: []*acm.CertificateSummary{
						{
							CertificateArn: aws.String("foobar"),
							DomainName:     aws.String("foobar.de"),
						},
					},
				},
				acm.GetCertificateOutput{
					Certificate: aws.String(cert),
				},
				map[string]*acm.ListTagsForCertificateOutput{
					"foobar": {
						Tags: []*acm.Tag{{Key: aws.String("production"), Value: aws.String("false")}},
					},
				},
			),
			filterTag: "production=true",
			expect: acmExpect{
				EmptyList:   true,
				ARN:         "foobar",
				DomainNames: []string{"foobar.de"},
				Error:       nil,
			},
		},
	} {
		t.Run(ti.msg, func(t *testing.T) {
			provider := newACMCertProvider(ti.api, ti.filterTag)
			list, err := provider.GetCertificates()

			if ti.expect.Error != nil {
				require.Equal(t, ti.expect.Error, err)
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
