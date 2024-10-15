package aws

import (
	"crypto/x509"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/acm"
	"github.com/aws/aws-sdk-go/service/acm/acmiface"
	"github.com/zalando-incubator/kube-ingress-aws-controller/certs"
)

type acmCertificateProvider struct {
	api       acmiface.ACMAPI
	filterTag string
}

func newACMCertProvider(api acmiface.ACMAPI, certFilterTag string) certs.CertificatesProvider {
	return &acmCertificateProvider{api: api, filterTag: certFilterTag}
}

// GetCertificates returns a list of AWS ACM certificates
func (p *acmCertificateProvider) GetCertificates() ([]*certs.CertificateSummary, error) {
	acmSummaries, err := getACMCertificateSummaries(p.api, p.filterTag)
	if err != nil {
		return nil, err
	}
	result := make([]*certs.CertificateSummary, 0)
	for _, o := range acmSummaries {
		summary, err := getCertificateSummaryFromACM(p.api, o.CertificateArn)
		if err != nil {
			return nil, err
		}
		result = append(result, summary)
	}
	return result, nil
}

func getACMCertificateSummaries(api acmiface.ACMAPI, filterTag string) ([]*acm.CertificateSummary, error) {
	params := &acm.ListCertificatesInput{
		CertificateStatuses: []*string{
			aws.String(acm.CertificateStatusIssued),
		},
	}
	acmSummaries := make([]*acm.CertificateSummary, 0)

	if err := api.ListCertificatesPages(params, func(page *acm.ListCertificatesOutput, lastPage bool) bool {
		acmSummaries = append(acmSummaries, page.CertificateSummaryList...)
		return true
	}); err != nil {
		return nil, err
	}

	if tag := strings.Split(filterTag, "="); filterTag != "=" && len(tag) == 2 {
		return filterCertificatesByTag(api, acmSummaries, tag[0], tag[1])
	}

	return acmSummaries, nil
}

func filterCertificatesByTag(api acmiface.ACMAPI, allSummaries []*acm.CertificateSummary, key, value string) ([]*acm.CertificateSummary, error) {
	prodSummaries := make([]*acm.CertificateSummary, 0)
	for _, summary := range allSummaries {
		in := &acm.ListTagsForCertificateInput{
			CertificateArn: summary.CertificateArn,
		}
		out, err := api.ListTagsForCertificate(in)
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

func getCertificateSummaryFromACM(api acmiface.ACMAPI, arn *string) (*certs.CertificateSummary, error) {
	params := &acm.GetCertificateInput{CertificateArn: arn}
	resp, err := api.GetCertificate(params)
	if err != nil {
		return nil, err
	}

	cert, err := ParseCertificate(aws.StringValue(resp.Certificate))
	if err != nil {
		return nil, err
	}

	var chain []*x509.Certificate
	if resp.CertificateChain != nil {
		chain, err = ParseCertificates(aws.StringValue(resp.CertificateChain))
		if err != nil {
			return nil, err
		}
	}

	return certs.NewCertificate(aws.StringValue(arn), cert, chain), nil
}
