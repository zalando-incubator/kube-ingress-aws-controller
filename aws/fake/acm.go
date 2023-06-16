package fake

import (
	"github.com/aws/aws-sdk-go/service/acm"
	"github.com/aws/aws-sdk-go/service/acm/acmiface"
)

type ACMClient struct {
	acmiface.ACMAPI
	output acm.ListCertificatesOutput
	cert   acm.GetCertificateOutput
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

func NewACMClient(output acm.ListCertificatesOutput, cert acm.GetCertificateOutput) ACMClient {
	return ACMClient{
		output: output,
		cert:   cert,
	}
}
