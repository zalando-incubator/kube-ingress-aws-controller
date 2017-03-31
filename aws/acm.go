package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/acm"
	"github.com/aws/aws-sdk-go/service/acm/acmiface"
)

type acmCertificateProvider struct {
	api acmiface.ACMAPI
}

func newACMCertProvider(api acmiface.ACMAPI) CertificatesProvider {
	return &acmCertificateProvider{api: api}
}

// GetCertificates returns a list of ACM certificates
func (p *acmCertificateProvider) GetCertificates() ([]*CertDetail, error) {
	certList, err := listCertsFromACM(p.api)
	if err != nil {
		return nil, err
	}
	result := make([]*CertDetail, 0)
	for _, o := range certList {
		certDetail, err := getACMCertDetails(p.api, aws.StringValue(o.CertificateArn))
		if err != nil {
			return nil, err
		}
		result = append(result, certDetail)
	}
	return result, nil
}

func listCertsFromACM(api acmiface.ACMAPI) ([]*acm.CertificateSummary, error) {
	certList := make([]*acm.CertificateSummary, 0)
	params := &acm.ListCertificatesInput{
		CertificateStatuses: []*string{
			aws.String(acm.CertificateStatusIssued), // Required
		},
	}
	err := api.ListCertificatesPages(params, func(page *acm.ListCertificatesOutput, lastPage bool) bool {
		for _, cert := range page.CertificateSummaryList {
			certList = append(certList, cert)
		}
		return true
	})
	return certList, err
}

func getACMCertDetails(api acmiface.ACMAPI, arn string) (*CertDetail, error) {
	params := &acm.DescribeCertificateInput{CertificateArn: aws.String(arn)}
	resp, err := api.DescribeCertificate(params)
	if err != nil {
		return nil, err
	}
	return certDetailFromACM(resp.Certificate), nil
}

func certDetailFromACM(acmDetail *acm.CertificateDetail) *CertDetail {
	var altNames []string
	for _, alt := range acmDetail.SubjectAlternativeNames {
		altNames = append(altNames, aws.StringValue(alt))
	}
	return &CertDetail{
		Arn:       aws.StringValue(acmDetail.CertificateArn),
		AltNames:  append(altNames, aws.StringValue(acmDetail.DomainName)),
		NotBefore: aws.TimeValue(acmDetail.NotBefore),
		NotAfter:  aws.TimeValue(acmDetail.NotAfter),
	}
}
