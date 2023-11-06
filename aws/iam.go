package aws

import (
	"crypto/x509"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/zalando-incubator/kube-ingress-aws-controller/certs"
)

type iamCertificateProvider struct {
	api       iamiface.IAMAPI
	filterTag string
}

func newIAMCertProvider(api iamiface.IAMAPI, filterTag string) certs.CertificatesProvider {
	return &iamCertificateProvider{api: api, filterTag: filterTag}
}

// GetCertificates returns a list of AWS IAM certificates
func (p *iamCertificateProvider) GetCertificates() ([]*certs.CertificateSummary, error) {
	serverCertificatesMetadata, err := getIAMServerCertificateMetadata(p.api)
	if err != nil {
		return nil, err
	}
	list := make([]*certs.CertificateSummary, 0)
	for _, o := range serverCertificatesMetadata {
		if kv := strings.Split(p.filterTag, "="); p.filterTag != "=" && len(kv) == 2 {
			hasTag, err := certHasTag(p.api, *o.ServerCertificateName, kv[0], kv[1])
			if err != nil {
				return nil, err
			}

			if !hasTag {
				continue
			}
		}

		certDetail, err := getCertificateSummaryFromIAM(p.api, aws.StringValue(o.ServerCertificateName))
		if err != nil {
			return nil, err
		}
		list = append(list, certDetail)
	}
	return list, nil
}

func certHasTag(api iamiface.IAMAPI, certName, key, value string) (bool, error) {
	t, err := api.ListServerCertificateTags(&iam.ListServerCertificateTagsInput{
		ServerCertificateName: &certName,
	})
	if err != nil {
		return false, err
	}
	for _, tag := range t.Tags {
		if *tag.Key == key && *tag.Value == value {
			return true, nil
		}
	}

	return false, nil
}

func getIAMServerCertificateMetadata(api iamiface.IAMAPI) ([]*iam.ServerCertificateMetadata, error) {
	params := &iam.ListServerCertificatesInput{
		PathPrefix: aws.String("/"),
	}
	certList := make([]*iam.ServerCertificateMetadata, 0)
	err := api.ListServerCertificatesPages(params, func(p *iam.ListServerCertificatesOutput, lastPage bool) bool {
		certList = append(certList, p.ServerCertificateMetadataList...)
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
	cert, err := ParseCertificate(aws.StringValue(iamCertDetail.CertificateBody))
	if err != nil {
		return nil, err
	}

	var chain []*x509.Certificate
	if iamCertDetail.CertificateChain != nil {
		chain, err = ParseCertificates(aws.StringValue(iamCertDetail.CertificateChain))
		if err != nil {
			return nil, err
		}
	}

	return certs.NewCertificate(
		aws.StringValue(iamCertDetail.ServerCertificateMetadata.Arn),
		cert,
		chain), nil
}
