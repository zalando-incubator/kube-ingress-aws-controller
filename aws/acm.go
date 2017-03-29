package aws

import (
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/service/acm"
	"github.com/aws/aws-sdk-go/service/acm/acmiface"
)

type acmClient struct {
	c acmiface.ACMAPI
}

func newACMClient(p client.ConfigProvider) *acmClient {
	return &acmClient{c: acm.New(p)}
}

func (c *acmClient) updateCerts(list []*CertDetail, errReturn *error, wg *sync.WaitGroup) {
	defer wg.Done()
	certList, err := c.listCerts()
	if err != nil {
		errReturn = &err
		return
	}

	for _, o := range certList {
		certInput := &acm.DescribeCertificateInput{CertificateArn: o.CertificateArn}
		certDetail, err := c.c.DescribeCertificate(certInput)
		if err != nil {
			errReturn = &err
			return
		}
		list = append(list, c.newCertDetail(certDetail.Certificate))
	}
}

// listCerts returns a list of acm Certificates filtered by
// CertificateStatuses
// https://docs.aws.amazon.com/acm/latest/APIReference/API_ListCertificates.html#API_ListCertificates_RequestSyntax
func (c *acmClient) listCerts() ([]*acm.CertificateSummary, error) {
	certList := make([]*acm.CertificateSummary, 0)
	params := &acm.ListCertificatesInput{
		CertificateStatuses: []*string{
			aws.String("ISSUED"), // Required
		},
	}
	err := c.c.ListCertificatesPages(params, func(page *acm.ListCertificatesOutput, lastPage bool) bool {
		for _, cert := range page.CertificateSummaryList {
			certList = append(certList, cert)
		}
		return true
	})
	return certList, err
}

func (c *acmClient) newCertDetail(acmDetail *acm.CertificateDetail) *CertDetail {
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
