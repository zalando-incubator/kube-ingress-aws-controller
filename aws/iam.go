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
	return &iamCertificateProvider{api: api}
}

func (p *iamCertificateProvider) GetCertificates() ([]*CertDetail, error) {
	certList, err := listCertsFromIAM(p.api)
	if err != nil {
		return nil, err
	}
	list := make([]*CertDetail, 0)
	for _, o := range certList {
		certInput := &iam.GetServerCertificateInput{ServerCertificateName: o.ServerCertificateName}
		cert, err := p.api.GetServerCertificate(certInput)
		if err != nil {
			return nil, err
		}
		certDetail := certDetailFromIAM(cert.ServerCertificate)
		if certDetail != nil {
			list = append(list, certDetail)

		}
	}
	return list, nil
}

// listCerts returns a list of iam certificates filtered by Path / for all ELB/ELBv2 certificate
// https://docs.aws.amazon.com/IAM/latest/APIReference/API_ListServerCertificates.html#API_ListServerCertificates_RequestParameters
func listCertsFromIAM(api iamiface.IAMAPI) ([]*iam.ServerCertificateMetadata, error) {
	certList := make([]*iam.ServerCertificateMetadata, 0)
	params := &iam.ListServerCertificatesInput{
		PathPrefix: aws.String("/"),
	}
	err := api.ListServerCertificatesPages(params, func(p *iam.ListServerCertificatesOutput, lastPage bool) bool {
		for _, cert := range p.ServerCertificateMetadataList {
			certList = append(certList, cert)
		}
		return true
	})
	return certList, err
}

func certDetailFromIAM(iamCertDetail *iam.ServerCertificate) *CertDetail {
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
	dnsNames := make([]string, 0)
	dnsNames = append(dnsNames, cert.DNSNames...)
	if cert.Subject.CommonName != "" {
		dnsNames = append(dnsNames, cert.Subject.CommonName)
	}
	return &CertDetail{
		Arn:       aws.StringValue(iamCertDetail.ServerCertificateMetadata.Arn),
		AltNames:  dnsNames,
		NotBefore: cert.NotBefore,
		NotAfter:  cert.NotAfter,
	}
}
