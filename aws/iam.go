package aws

import (
	"crypto/x509"
	"encoding/pem"
	"log"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
)

type iamClient struct {
	c iamiface.IAMAPI
}

func newIAMClient(p client.ConfigProvider) *iamClient {
	return &iamClient{c: iam.New(p)}
}

func (c *iamClient) updateCerts(list []*CertDetail, errReturn *error, wg *sync.WaitGroup) {
	defer wg.Done()
	certList, err := c.listCerts()
	if err != nil {
		errReturn = &err
		return
	}

	for _, o := range certList {
		certInput := &iam.GetServerCertificateInput{ServerCertificateName: o.ServerCertificateName}
		certDetail, err := c.c.GetServerCertificate(certInput)
		if err != nil {
			errReturn = &err
			return
		}
		list = append(list, c.newCertDetail(certDetail.ServerCertificate))
	}
}

// listCerts returns a list of iam certificates filtered by Path / for all ELB/ELBv2 certificate
// https://docs.aws.amazon.com/IAM/latest/APIReference/API_ListServerCertificates.html#API_ListServerCertificates_RequestParameters
func (c *iamClient) listCerts() ([]*iam.ServerCertificateMetadata, error) {
	certList := make([]*iam.ServerCertificateMetadata, 0)
	params := &iam.ListServerCertificatesInput{
		PathPrefix: aws.String("/"),
	}
	err := c.c.ListServerCertificatesPages(params, func(p *iam.ListServerCertificatesOutput, lastPage bool) bool {
		for _, cert := range p.ServerCertificateMetadataList {
			certList = append(certList, cert)
		}
		return true
	})
	return certList, err
}

func (c *iamClient) newCertDetail(iamCertDetail *iam.ServerCertificate) *CertDetail {
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
