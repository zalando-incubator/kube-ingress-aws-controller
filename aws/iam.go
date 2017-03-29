package aws

import (
	"crypto/x509"
	"encoding/pem"
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
)

type iamCertificateProvider struct {
	api iamiface.IAMAPI
}

func newIAMCertProvider(api iamiface.IAMAPI) CertificatesProvider {
	return iamCertificateProvider{api: api}
}

func (p iamCertificateProvider) GetCertificates() ([]*CertDetail, error) {
	certList, err := p.listCerts()
	if err != nil {
		return nil, err
	}
	list := make([]*CertDetail, 0)
	for _, o := range certList {
		certInput := &iam.GetServerCertificateInput{ServerCertificateName: o.ServerCertificateName}
		certDetail, err := p.api.GetServerCertificate(certInput)
		if err != nil {
			return nil, err
		}
		list = append(list, p.newCertDetail(certDetail.ServerCertificate))
	}
	return list, nil
}

// listCerts returns a list of iam certificates filtered by Path / for all ELB/ELBv2 certificate
// https://docs.aws.amazon.com/IAM/latest/APIReference/API_ListServerCertificates.html#API_ListServerCertificates_RequestParameters
func (p iamCertificateProvider) listCerts() ([]*iam.ServerCertificateMetadata, error) {
	certList := make([]*iam.ServerCertificateMetadata, 0)
	params := &iam.ListServerCertificatesInput{
		PathPrefix: aws.String("/"),
	}
	err := p.api.ListServerCertificatesPages(params, func(p *iam.ListServerCertificatesOutput, lastPage bool) bool {
		for _, cert := range p.ServerCertificateMetadataList {
			certList = append(certList, cert)
		}
		return true
	})
	return certList, err
}

func (p iamCertificateProvider) newCertDetail(iamCertDetail *iam.ServerCertificate) *CertDetail {
	block, _ := pem.Decode([]byte(*iamCertDetail.CertificateBody))
	if block == nil {
		log.Println("failed to parse certificate PEM")
		return nil
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		log.Println(err)
		return nil
	}
	return &CertDetail{
		Arn:       aws.StringValue(iamCertDetail.ServerCertificateMetadata.Arn),
		AltNames:  append(cert.DNSNames, cert.Subject.CommonName),
		NotBefore: cert.NotBefore,
		NotAfter:  cert.NotAfter,
	}
}
