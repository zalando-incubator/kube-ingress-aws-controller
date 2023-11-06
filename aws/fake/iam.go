package fake

import (
	"fmt"

	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
)

type IAMClient struct {
	iamiface.IAMAPI
	list iam.ListServerCertificatesOutput
	cert iam.GetServerCertificateOutput
	tags map[string]*iam.ListServerCertificateTagsOutput
}

func (m IAMClient) ListServerCertificates(*iam.ListServerCertificatesInput) (*iam.ListServerCertificatesOutput, error) {
	return &m.list, nil
}

func (m IAMClient) ListServerCertificatesPages(input *iam.ListServerCertificatesInput, fn func(*iam.ListServerCertificatesOutput, bool) bool) error {
	fn(&m.list, true)
	return nil
}

func (m IAMClient) ListServerCertificateTags(
	in *iam.ListServerCertificateTagsInput,
) (*iam.ListServerCertificateTagsOutput, error) {

	if in.ServerCertificateName == nil {
		return nil, fmt.Errorf("expected a valid CertificateArn, got: nil")
	}
	name := *in.ServerCertificateName
	return m.tags[name], nil
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

func NewIAMClientWithTag(
	list iam.ListServerCertificatesOutput,
	cert iam.GetServerCertificateOutput,
	tags map[string]*iam.ListServerCertificateTagsOutput,
) IAMClient {
	return IAMClient{
		list: list,
		cert: cert,
		tags: tags,
	}
}
