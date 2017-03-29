package aws

import (
	"reflect"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/acm"
	"github.com/aws/aws-sdk-go/service/acm/acmiface"
	"github.com/davecgh/go-spew/spew"
)

type mockedACMClient struct {
	acmiface.ACMAPI
	output acm.ListCertificatesOutput
	cert   acm.DescribeCertificateOutput
}

func (m mockedACMClient) ListCertificates(in *acm.ListCertificatesInput) (*acm.ListCertificatesOutput, error) {
	return &m.output, nil
}

func (m mockedACMClient) ListCertificatesPages(input *acm.ListCertificatesInput, fn func(p *acm.ListCertificatesOutput, lastPage bool) (shouldContinue bool)) error {
	fn(&m.output, true)
	return nil
}

func (m mockedACMClient) DescribeCertificate(input *acm.DescribeCertificateInput) (*acm.DescribeCertificateOutput, error) {
	return &m.cert, nil
}

type acmExpect struct {
	List  []*CertDetail
	Error error
}

func TestACM(t *testing.T) {
	now := time.Now()
	before := now.Add(-time.Hour * 24 * 7)
	after := now.Add(time.Hour*24*7 + 1*time.Second)
	for _, ti := range []struct {
		msg      string
		provider acmCertificateProvider
		expect   acmExpect
	}{
		{
			msg: "Found ACM Cert foobar",
			provider: acmCertificateProvider{
				api: mockedACMClient{
					output: acm.ListCertificatesOutput{
						CertificateSummaryList: []*acm.CertificateSummary{
							{
								CertificateArn: aws.String("foobar"),
								DomainName:     aws.String("foobar.de"),
							},
						},
					},
					cert: acm.DescribeCertificateOutput{
						Certificate: &acm.CertificateDetail{
							CertificateArn: aws.String("foobar"),
							DomainName:     aws.String("foobar.de"),
							NotAfter:       aws.Time(after),
							NotBefore:      aws.Time(before),
							SubjectAlternativeNames: []*string{
								aws.String("foobar.de"),
							},
						},
					},
				},
			},
			expect: acmExpect{
				List: []*CertDetail{
					{
						NotAfter:  after,
						NotBefore: before,
						Arn:       "foobar",
						AltNames:  []string{"foobar.de", "foobar.de"},
					},
				},
				Error: nil,
			},
		},
	} {
		t.Run(ti.msg, func(t *testing.T) {
			list, err := ti.provider.GetCertificates()
			if !reflect.DeepEqual(list, ti.expect.List) {
				t.Errorf("%s:\nexpected %s\ngiven: %s\n", ti.msg, spew.Sdump(ti.expect.List), spew.Sdump(list))
			}
			if err != ti.expect.Error {
				t.Errorf("%s: expected %#v\ngiven: %#v\n", ti.msg, ti.expect.Error, err)
			}
		})
	}
}
