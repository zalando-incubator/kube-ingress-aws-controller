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
	return acmCertificateProvider{api: api}
}

func (p acmCertificateProvider) GetCertificates() ([]*CertDetail, error) {
	certList, err := p.listCerts()
	if err != nil {
		return nil, err
	}
	list := make([]*CertDetail, 0)
	for _, o := range certList {
		certInput := &acm.DescribeCertificateInput{CertificateArn: o.CertificateArn}
		certDetail, err := p.api.DescribeCertificate(certInput)
		if err != nil {
			return nil, err
		}
		list = append(list, p.newCertDetail(certDetail.Certificate))
	}
	return list, nil
}

// listCerts returns a list of acm Certificates filtered by
// CertificateStatuses
// https://docs.aws.amazon.com/acm/latest/APIReference/API_ListCertificates.html#API_ListCertificates_RequestSyntax
func (p acmCertificateProvider) listCerts() ([]*acm.CertificateSummary, error) {
	certList := make([]*acm.CertificateSummary, 0)
	params := &acm.ListCertificatesInput{
		CertificateStatuses: []*string{
			aws.String("ISSUED"), // Required
		},
	}
	err := p.api.ListCertificatesPages(params, func(page *acm.ListCertificatesOutput, lastPage bool) bool {
		for _, cert := range page.CertificateSummaryList {
			certList = append(certList, cert)
		}
		return true
	})
	return certList, err
}

func (p acmCertificateProvider) newCertDetail(acmDetail *acm.CertificateDetail) *CertDetail {
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
