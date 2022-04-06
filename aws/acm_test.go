package aws

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/acm"
	"github.com/aws/aws-sdk-go/service/acm/acmiface"
	"github.com/stretchr/testify/require"
)

type mockedACMClient struct {
	acmiface.ACMAPI
	output acm.ListCertificatesOutput
	cert   acm.GetCertificateOutput
}

func (m mockedACMClient) ListCertificates(in *acm.ListCertificatesInput) (*acm.ListCertificatesOutput, error) {
	return &m.output, nil
}

func (m mockedACMClient) ListCertificatesPages(input *acm.ListCertificatesInput, fn func(p *acm.ListCertificatesOutput, lastPage bool) (shouldContinue bool)) error {
	fn(&m.output, true)
	return nil
}

func (m mockedACMClient) GetCertificate(input *acm.GetCertificateInput) (*acm.GetCertificateOutput, error) {
	return &m.cert, nil
}

type acmExpect struct {
	ARN         string
	DomainNames []string
	Chain       int
	Error       error
}

func TestACM(t *testing.T) {
	cert := mustRead("acm.txt")
	chain := mustRead("chain.txt")

	for _, ti := range []struct {
		msg    string
		api    acmiface.ACMAPI
		expect acmExpect
	}{
		{
			msg: "Found ACM Cert foobar and a chain",
			api: mockedACMClient{
				output: acm.ListCertificatesOutput{
					CertificateSummaryList: []*acm.CertificateSummary{
						{
							CertificateArn: aws.String("foobar"),
							DomainName:     aws.String("foobar.de"),
						},
					},
				},
				cert: acm.GetCertificateOutput{
					Certificate:      aws.String(cert),
					CertificateChain: aws.String(chain),
				},
			},
			expect: acmExpect{
				ARN:         "foobar",
				DomainNames: []string{"foobar.de"},
				Error:       nil,
			},
		},
		{
			msg: "Found ACM Cert foobar and no chain",
			api: mockedACMClient{
				output: acm.ListCertificatesOutput{
					CertificateSummaryList: []*acm.CertificateSummary{
						{
							CertificateArn: aws.String("foobar"),
							DomainName:     aws.String("foobar.de"),
						},
					},
				},
				cert: acm.GetCertificateOutput{
					Certificate: aws.String(cert),
				},
			},
			expect: acmExpect{
				ARN:         "foobar",
				DomainNames: []string{"foobar.de"},
				Error:       nil,
			},
		},
	} {
		t.Run(ti.msg, func(t *testing.T) {
			provider := newACMCertProvider(ti.api)
			list, err := provider.GetCertificates()

			if ti.expect.Error != nil {
				require.Equal(t, ti.expect.Error, err)
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, 1, len(list))

			cert := list[0]
			require.Equal(t, ti.expect.ARN, cert.ID())
			require.Equal(t, ti.expect.DomainNames, cert.DomainNames())
		})
	}
}
