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
	err  map[string]error
}

func (m *ACMClient) ListCertificates(ctx context.Context, input *acm.ListCertificatesInput, fn ...func(*acm.Options)) (*acm.ListCertificatesOutput, error) {
	if m.err != nil {
		if err, ok := m.err["ListCertificates"]; ok {
			return nil, err
		}
	}
	output := &acm.ListCertificatesOutput{
		CertificateSummaryList: make([]types.CertificateSummary, 0),
	}
	for arn := range m.cert {
		output.CertificateSummaryList = append(output.CertificateSummaryList, types.CertificateSummary{
			CertificateArn: &arn,
		})
	}
	return output, nil
}

func (m *ACMClient) GetCertificate(ctx context.Context, input *acm.GetCertificateInput, fn ...func(*acm.Options)) (*acm.GetCertificateOutput, error) {
	if m.err != nil {
		if err, ok := m.err["GetCertificate"]; ok {
			return nil, err
		}
	}
	if input.CertificateArn == nil {
		return nil, fmt.Errorf("expected a valid CertificateArn, got: nil")
	}

	arn := *input.CertificateArn
	if _, ok := m.cert[arn]; ok {
		return m.cert[arn], nil
	}
	return nil, fmt.Errorf("cert not found: %s", arn)
}

func (m *ACMClient) ListTagsForCertificate(ctx context.Context, in *acm.ListTagsForCertificateInput, fn ...func(*acm.Options)) (*acm.ListTagsForCertificateOutput, error) {
	if m.err != nil {
		if err, ok := m.err["ListTagsForCertificate"]; ok {
			return nil, err
		}
	}
	if in.CertificateArn == nil {
		return nil, fmt.Errorf("expected a valid CertificateArn, got: nil")
	}
	arn := *in.CertificateArn
	return m.tags[arn], nil
}

func NewACMClient(
	cert map[string]*acm.GetCertificateOutput,
	tags map[string]*acm.ListTagsForCertificateOutput,
	err map[string]error,
) *ACMClient {
	c := &ACMClient{
		cert: cert,
		tags: tags,
		err:  err,
	}
	return c
}
