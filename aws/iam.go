package aws

import (
	"context"
	"crypto/x509"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/zalando-incubator/kube-ingress-aws-controller/certs"
)

type IAMIFaceAPI interface {
	GetServerCertificate(context.Context, *iam.GetServerCertificateInput, ...func(*iam.Options)) (*iam.GetServerCertificateOutput, error)
	ListServerCertificateTags(context.Context, *iam.ListServerCertificateTagsInput, ...func(*iam.Options)) (*iam.ListServerCertificateTagsOutput, error)
	ListServerCertificates(context.Context, *iam.ListServerCertificatesInput, ...func(*iam.Options)) (*iam.ListServerCertificatesOutput, error)
}

type iamCertificateProvider struct {
	api       IAMIFaceAPI
	filterTag string
}

func newIAMCertProvider(api IAMIFaceAPI, filterTag string) certs.CertificatesProvider {
	return &iamCertificateProvider{api: api, filterTag: filterTag}
}

// GetCertificates returns a list of AWS IAM certificates
func (p *iamCertificateProvider) GetCertificates() ([]*certs.CertificateSummary, error) {
	ctx := context.Background()
	serverCertificatesMetadata, err := getIAMServerCertificateMetadata(p.api)
	if err != nil {
		return nil, err
	}
	list := make([]*certs.CertificateSummary, 0)
	for _, o := range serverCertificatesMetadata {
		if kv := strings.Split(p.filterTag, "="); p.filterTag != "=" && len(kv) == 2 {
			hasTag, err := certHasTag(ctx, p.api, *o.ServerCertificateName, kv[0], kv[1])
			if err != nil {
				return nil, err
			}

			if !hasTag {
				continue
			}
		}

		certDetail, err := getCertificateSummaryFromIAM(ctx, p.api, aws.ToString(o.ServerCertificateName))
		if err != nil {
			return nil, err
		}
		list = append(list, certDetail)
	}
	return list, nil
}

func certHasTag(ctx context.Context, api IAMIFaceAPI, certName, key, value string) (bool, error) {
	t, err := api.ListServerCertificateTags(ctx, &iam.ListServerCertificateTagsInput{
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

func getIAMServerCertificateMetadata(api IAMIFaceAPI) ([]*types.ServerCertificateMetadata, error) {
	params := &iam.ListServerCertificatesInput{
		PathPrefix: aws.String("/"),
	}
	certList := make([]*types.ServerCertificateMetadata, 0)
	paginator := iam.NewListServerCertificatesPaginator(api, params)
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(context.TODO())
		if err != nil {
			return nil, err
		}
		for _, serverCertificateMetadata := range output.ServerCertificateMetadataList {
			certList = append(certList, &serverCertificateMetadata)
		}
	}
	return certList, nil
}

func getCertificateSummaryFromIAM(ctx context.Context, api IAMIFaceAPI, name string) (*certs.CertificateSummary, error) {
	params := &iam.GetServerCertificateInput{ServerCertificateName: aws.String(name)}
	resp, err := api.GetServerCertificate(ctx, params)
	if err != nil {
		return nil, err
	}
	certificateSummary, err := summaryFromServerCertificate(resp.ServerCertificate)
	if err != nil {
		return nil, err
	}
	return certificateSummary, nil
}

func summaryFromServerCertificate(iamCertDetail *types.ServerCertificate) (*certs.CertificateSummary, error) {
	cert, err := ParseCertificate(aws.ToString(iamCertDetail.CertificateBody))
	if err != nil {
		return nil, err
	}

	var chain []*x509.Certificate
	if iamCertDetail.CertificateChain != nil {
		chain, err = ParseCertificates(aws.ToString(iamCertDetail.CertificateChain))
		if err != nil {
			return nil, err
		}
	}

	return certs.NewCertificate(
		aws.ToString(iamCertDetail.ServerCertificateMetadata.Arn),
		cert,
		chain), nil
}
