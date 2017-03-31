package aws

import (
	"crypto/x509"
	"encoding/pem"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/pkg/errors"
)

type iamCertificateProvider struct {
	api iamiface.IAMAPI
}

var errFailedToParsePEM = errors.New("failed to parse certificate PEM")

func newIAMCertProvider(api iamiface.IAMAPI) CertificatesProvider {
	return &iamCertificateProvider{api: api}
}

// GetCertificates returns a list of IAM certificates
func (p *iamCertificateProvider) GetCertificates() ([]*CertDetail, error) {
	certList, err := listCertsFromIAM(p.api)
	if err != nil {
		return nil, err
	}
	list := make([]*CertDetail, 0)
	for _, o := range certList {
		certDetail, err := getIAMCertDetails(p.api, aws.StringValue(o.ServerCertificateName))
		if err != nil {
			return nil, err
		}
		list = append(list, certDetail)
	}
	return list, nil
}

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

func getIAMCertDetails(api iamiface.IAMAPI, name string) (*CertDetail, error) {
	params := &iam.GetServerCertificateInput{ServerCertificateName: aws.String(name)}
	resp, err := api.GetServerCertificate(params)
	if err != nil {
		return nil, err
	}
	certDetail, err := certDetailFromIAM(resp.ServerCertificate)
	if err != nil {
		return nil, err
	}
	return certDetail, nil
}

func certDetailFromIAM(iamCertDetail *iam.ServerCertificate) (*CertDetail, error) {
	block, _ := pem.Decode([]byte(*iamCertDetail.CertificateBody))
	if block == nil {
		return nil, errFailedToParsePEM
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
	return &CertDetail{
		Arn:       aws.StringValue(iamCertDetail.ServerCertificateMetadata.Arn),
		AltNames:  dnsNames,
		NotBefore: cert.NotBefore,
		NotAfter:  cert.NotAfter,
	}, nil
}
