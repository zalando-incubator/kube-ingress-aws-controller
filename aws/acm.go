package aws

import (
	"context"
	"crypto/x509"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	"github.com/aws/aws-sdk-go-v2/service/acm/types"
	"github.com/zalando-incubator/kube-ingress-aws-controller/certs"
)

type ACMAPI interface {
	acm.ListCertificatesAPIClient
	ListTagsForCertificate(context.Context, *acm.ListTagsForCertificateInput, ...func(*acm.Options)) (*acm.ListTagsForCertificateOutput, error)
	GetCertificate(context.Context, *acm.GetCertificateInput, ...func(*acm.Options)) (*acm.GetCertificateOutput, error)
}

type acmCertificateProvider struct {
	api       ACMAPI
	filterTag string
}

func newACMCertProvider(api ACMAPI, certFilterTag string) certs.CertificatesProvider {
	return &acmCertificateProvider{api: api, filterTag: certFilterTag}
}

// GetCertificates returns a list of AWS ACM certificates
func (p *acmCertificateProvider) GetCertificates(ctx context.Context) ([]*certs.CertificateSummary, error) {
	acmSummaries, err := getACMCertificateSummaries(ctx, p.api, p.filterTag)
	if err != nil {
		return nil, err
	}
	result := make([]*certs.CertificateSummary, 0, len(acmSummaries))
	for _, o := range acmSummaries {
		summary, err := getCertificateSummaryFromACM(ctx, p.api, o.CertificateArn)
		if err != nil {
			return nil, err
		}
		result = append(result, summary)
	}
	return result, nil
}

func getACMCertificateSummaries(ctx context.Context, api ACMAPI, filterTag string) ([]types.CertificateSummary, error) {
	params := &acm.ListCertificatesInput{
		CertificateStatuses: []types.CertificateStatus{
			types.CertificateStatusIssued,
		},
	}
	acmSummaries := make([]types.CertificateSummary, 0)

	paginator := acm.NewListCertificatesPaginator(api, params)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list certificates page: %w", err)
		}
		acmSummaries = append(acmSummaries, page.CertificateSummaryList...)
	}

	if tag := strings.Split(filterTag, "="); filterTag != "=" && len(tag) == 2 {
		return filterCertificatesByTag(ctx, api, acmSummaries, tag[0], tag[1])
	}

	return acmSummaries, nil
}

func filterCertificatesByTag(ctx context.Context, api ACMAPI, allSummaries []types.CertificateSummary, key, value string) ([]types.CertificateSummary, error) {
	prodSummaries := make([]types.CertificateSummary, 0)
	for _, summary := range allSummaries {
		in := &acm.ListTagsForCertificateInput{
			CertificateArn: summary.CertificateArn,
		}
		out, err := api.ListTagsForCertificate(ctx, in)
		if err != nil {
			return nil, err
		}

		for _, tag := range out.Tags {
			if *tag.Key == key && *tag.Value == value {
				prodSummaries = append(prodSummaries, summary)
			}
		}
	}

	return prodSummaries, nil
}

func getCertificateSummaryFromACM(ctx context.Context, api ACMAPI, arn *string) (*certs.CertificateSummary, error) {
	params := &acm.GetCertificateInput{CertificateArn: arn}
	resp, err := api.GetCertificate(ctx, params)
	if err != nil {
		return nil, err
	}

	cert, err := ParseCertificate(aws.ToString(resp.Certificate))
	if err != nil {
		return nil, err
	}

	var chain []*x509.Certificate
	if resp.CertificateChain != nil {
		chain, err = ParseCertificates(aws.ToString(resp.CertificateChain))
		if err != nil {
			return nil, err
		}
	}

	return certs.NewCertificate(aws.ToString(arn), cert, chain), nil
}
