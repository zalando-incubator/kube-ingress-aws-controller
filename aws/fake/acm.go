package fake

import (
	"fmt"

	"github.com/aws/aws-sdk-go/service/acm"
	"github.com/aws/aws-sdk-go/service/acm/acmiface"
)

type ACMClient struct {
	acmiface.ACMAPI
	output acm.ListCertificatesOutput
	cert   acm.GetCertificateOutput
	tags   map[string]*acm.ListTagsForCertificateOutput
}

func (m ACMClient) ListCertificates(in *acm.ListCertificatesInput) (*acm.ListCertificatesOutput, error) {
	return &m.output, nil
}

func (m ACMClient) ListCertificatesPages(input *acm.ListCertificatesInput, fn func(p *acm.ListCertificatesOutput, lastPage bool) (shouldContinue bool)) error {
	fn(&m.output, true)
	return nil
}

func (m ACMClient) GetCertificate(input *acm.GetCertificateInput) (*acm.GetCertificateOutput, error) {
	return &m.cert, nil
}

func (m ACMClient) ListTagsForCertificate(in *acm.ListTagsForCertificateInput) (*acm.ListTagsForCertificateOutput, error) {
	if in.CertificateArn == nil {
		return nil, fmt.Errorf("expected a valid CertificateArn, got: nil")
	}
	arn := *in.CertificateArn
	return m.tags[arn], nil
}

func NewACMClient(output acm.ListCertificatesOutput, cert acm.GetCertificateOutput) ACMClient {
	return ACMClient{
		output: output,
		cert:   cert,
	}
}

func NewACMClientWithTags(
	output acm.ListCertificatesOutput,
	cert acm.GetCertificateOutput,
	tags map[string]*acm.ListTagsForCertificateOutput,
) ACMClient {
	return ACMClient{
		output: output,
		cert:   cert,
		tags:   tags,
	}
}
