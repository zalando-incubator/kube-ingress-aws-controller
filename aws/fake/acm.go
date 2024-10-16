package fake

import (
	"fmt"

	"github.com/aws/aws-sdk-go/service/acm"
	"github.com/aws/aws-sdk-go/service/acm/acmiface"
)

type ACMClient struct {
	acmiface.ACMAPI
	cert map[string]*acm.GetCertificateOutput
	tags map[string]*acm.ListTagsForCertificateOutput

	listCertificatesPages func(input *acm.ListCertificatesInput, fn func(p *acm.ListCertificatesOutput, lastPage bool) (shouldContinue bool)) error
}

func (m *ACMClient) ListCertificatesPages(input *acm.ListCertificatesInput, fn func(p *acm.ListCertificatesOutput, lastPage bool) (shouldContinue bool)) error {
	return m.listCertificatesPages(input, fn)
}

func (m *ACMClient) GetCertificate(input *acm.GetCertificateInput) (*acm.GetCertificateOutput, error) {
	return m.cert[*input.CertificateArn], nil
}

func (m *ACMClient) ListTagsForCertificate(in *acm.ListTagsForCertificateInput) (*acm.ListTagsForCertificateOutput, error) {
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
