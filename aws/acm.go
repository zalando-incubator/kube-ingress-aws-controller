package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/acm"
	"github.com/aws/aws-sdk-go/service/acm/acmiface"
	"github.com/zalando-incubator/kube-ingress-aws-controller/certs"
)

type acmCertificateProvider struct {
	api acmiface.ACMAPI
}

func newACMCertProvider(api acmiface.ACMAPI) certs.CertificatesProvider {
	return &acmCertificateProvider{api: api}
}

// GetCertificates returns a list of AWS ACM certificates
func (p *acmCertificateProvider) GetCertificates() ([]*certs.CertificateSummary, error) {
	acmSummaries, err := getACMCertificateSummaries(p.api)
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

func getACMCertificateSummaries(api acmiface.ACMAPI) ([]*acm.CertificateSummary, error) {
	params := &acm.ListCertificatesInput{
		CertificateStatuses: []*string{
			aws.String(acm.CertificateStatusIssued),
		},
	}
	acmSummaries := make([]*acm.CertificateSummary, 0)
	err := api.ListCertificatesPages(params, func(page *acm.ListCertificatesOutput, lastPage bool) bool {
		for _, cert := range page.CertificateSummaryList {
			acmSummaries = append(acmSummaries, cert)
		}
		return true
	})
	return acmSummaries, err
}

func getCertificateSummaryFromACM(api acmiface.ACMAPI, arn *string) (*certs.CertificateSummary, error) {
	params := &acm.DescribeCertificateInput{CertificateArn: arn}
	resp, err := api.DescribeCertificate(params)
	if err != nil {
		return nil, err
	}
	return certs.NewCertificate(
		aws.StringValue(resp.Certificate.CertificateArn),
		append(aws.StringValueSlice(resp.Certificate.SubjectAlternativeNames), aws.StringValue(resp.Certificate.DomainName)),
		aws.TimeValue(resp.Certificate.NotBefore),
		aws.TimeValue(resp.Certificate.NotAfter)), nil
}
