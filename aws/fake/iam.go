package fake

import (
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
)

type IAMClient struct {
	iamiface.IAMAPI
	list iam.ListServerCertificatesOutput
	cert iam.GetServerCertificateOutput
}

func (m IAMClient) ListServerCertificates(*iam.ListServerCertificatesInput) (*iam.ListServerCertificatesOutput, error) {
	return &m.list, nil
}

func (m IAMClient) ListServerCertificatesPages(input *iam.ListServerCertificatesInput, fn func(*iam.ListServerCertificatesOutput, bool) bool) error {
	fn(&m.list, true)
	return nil
}

func (m IAMClient) GetServerCertificate(*iam.GetServerCertificateInput) (*iam.GetServerCertificateOutput, error) {
	return &m.cert, nil
}

func NewIAMClient(list iam.ListServerCertificatesOutput, cert iam.GetServerCertificateOutput) IAMClient {
	return IAMClient{
		list: list,
		cert: cert,
	}
}
