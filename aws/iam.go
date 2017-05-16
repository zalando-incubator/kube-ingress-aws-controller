package aws

import (
	"crypto/x509"
	"encoding/pem"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/zalando-incubator/kube-ingress-aws-controller/certs"
)

type iamCertificateProvider struct {
	api iamiface.IAMAPI
}

func newIAMCertProvider(api iamiface.IAMAPI) certs.CertificatesProvider {
	return &iamCertificateProvider{api: api}
}

// GetCertificates returns a list of AWS IAM certificates
func (p *iamCertificateProvider) GetCertificates() ([]*certs.CertificateSummary, error) {
	serverCertificatesMetadata, err := getIAMServerCertificateMetadata(p.api)
	if err != nil {
		return nil, err
	}
	list := make([]*certs.CertificateSummary, 0)
	for _, o := range serverCertificatesMetadata {
		certDetail, err := getCertificateSummaryFromIAM(p.api, aws.StringValue(o.ServerCertificateName))
		if err != nil {
			return nil, err
		}
		list = append(list, certDetail)
	}
	return list, nil
}

func getIAMServerCertificateMetadata(api iamiface.IAMAPI) ([]*iam.ServerCertificateMetadata, error) {
	params := &iam.ListServerCertificatesInput{
		PathPrefix: aws.String("/"),
	}
	certList := make([]*iam.ServerCertificateMetadata, 0)
	err := api.ListServerCertificatesPages(params, func(p *iam.ListServerCertificatesOutput, lastPage bool) bool {
		for _, cert := range p.ServerCertificateMetadataList {
			certList = append(certList, cert)
		}
		return true
	})
	return certList, err
}

func getCertificateSummaryFromIAM(api iamiface.IAMAPI, name string) (*certs.CertificateSummary, error) {
	params := &iam.GetServerCertificateInput{ServerCertificateName: aws.String(name)}
	resp, err := api.GetServerCertificate(params)
	if err != nil {
		return nil, err
	}
	certificateSummary, err := summaryFromServerCertificate(resp.ServerCertificate)
	if err != nil {
		return nil, err
	}
	return certificateSummary, nil
}

func summaryFromServerCertificate(iamCertDetail *iam.ServerCertificate) (*certs.CertificateSummary, error) {
	block, _ := pem.Decode([]byte(*iamCertDetail.CertificateBody))
	if block == nil {
		return nil, ErrFailedToParsePEM
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}

	dnsNames := make([]string, 0)
	dnsNames = append(dnsNames, cert.DNSNames...)
	if cert.Subject.CommonName != "" {
		dnsNames = append(dnsNames, cert.Subject.CommonName)
	}
	return certs.NewCertificate(
		aws.StringValue(iamCertDetail.ServerCertificateMetadata.Arn),
		dnsNames,
		cert.NotBefore,
		cert.NotAfter), nil
}
