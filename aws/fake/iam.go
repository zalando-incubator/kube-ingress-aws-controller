package fake

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/iam"
)

type IAMClient struct {
	list iam.ListServerCertificatesOutput
	cert iam.GetServerCertificateOutput
	tags map[string]*iam.ListServerCertificateTagsOutput
}

func (m IAMClient) ListServerCertificates(context.Context, *iam.ListServerCertificatesInput, ...func(*iam.Options)) (*iam.ListServerCertificatesOutput, error) {
	return &m.list, nil
}

func (m IAMClient) ListServerCertificateTags(ctx context.Context, in *iam.ListServerCertificateTagsInput, fn ...func(*iam.Options)) (*iam.ListServerCertificateTagsOutput, error) {
	if in.ServerCertificateName == nil {
		return nil, fmt.Errorf("expected a valid CertificateArn, got: nil")
	}
	name := *in.ServerCertificateName
	return m.tags[name], nil
}

func (m IAMClient) GetServerCertificate(context.Context, *iam.GetServerCertificateInput, ...func(*iam.Options)) (*iam.GetServerCertificateOutput, error) {
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
