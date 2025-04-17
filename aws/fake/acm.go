package fake

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/acm"
	"github.com/aws/aws-sdk-go-v2/service/acm/types"
)

type ACMClient struct {
	cert map[string]*acm.GetCertificateOutput
	tags map[string]*acm.ListTagsForCertificateOutput

	listCertificatesPages func(input *acm.ListCertificatesInput, fn func(p *acm.ListCertificatesOutput, lastPage bool) (shouldContinue bool)) error
}

func (m *ACMClient) ListCertificatesPages(input *acm.ListCertificatesInput, fn func(p *acm.ListCertificatesOutput, lastPage bool) (shouldContinue bool)) error {
	return m.listCertificatesPages(input, fn)
}

func (m *ACMClient) ListCertificates(ctx context.Context, input *acm.ListCertificatesInput, fn ...func(*acm.Options)) (*acm.ListCertificatesOutput, error) {
	output := &acm.ListCertificatesOutput{
		CertificateSummaryList: make([]types.CertificateSummary, 0),
	}
	for _, cert := range m.cert {
		output.CertificateSummaryList = append(output.CertificateSummaryList, types.CertificateSummary{
			CertificateArn: cert.Certificate,
		})
	}
	return output, nil
}

func (m *ACMClient) GetCertificate(ctx context.Context, input *acm.GetCertificateInput, fn ...func(*acm.Options)) (*acm.GetCertificateOutput, error) {
	return m.cert[*input.CertificateArn], nil
}

func (m *ACMClient) ListTagsForCertificate(ctx context.Context, in *acm.ListTagsForCertificateInput, fn ...func(*acm.Options)) (*acm.ListTagsForCertificateOutput, error) {
	if in.CertificateArn == nil {
		return nil, fmt.Errorf("expected a valid CertificateArn, got: nil")
	}
	arn := *in.CertificateArn
	return m.tags[arn], nil
}

func (m *ACMClient) WithListCertificatesPages(f func(input *acm.ListCertificatesInput, fn func(p *acm.ListCertificatesOutput, lastPage bool) (shouldContinue bool)) error) *ACMClient {
	m.listCertificatesPages = f
	return m
}

func NewACMClient(
	output acm.ListCertificatesOutput,
	cert map[string]*acm.GetCertificateOutput,
	tags map[string]*acm.ListTagsForCertificateOutput,
) *ACMClient {
	c := &ACMClient{
		cert: cert,
		tags: tags,
	}
	c.WithListCertificatesPages(func(input *acm.ListCertificatesInput, fn func(p *acm.ListCertificatesOutput, lastPage bool) (shouldContinue bool)) error {
		fn(&output, true)
		return nil
	})
	return c
}
