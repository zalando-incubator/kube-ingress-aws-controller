package aws

import (
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/stretchr/testify/require"
)

type mockedIAMClient struct {
	iamiface.IAMAPI
	list iam.ListServerCertificatesOutput
	cert iam.GetServerCertificateOutput
}

func (m mockedIAMClient) ListServerCertificates(*iam.ListServerCertificatesInput) (*iam.ListServerCertificatesOutput, error) {
	return &m.list, nil
}

func (m mockedIAMClient) ListServerCertificatesPages(input *iam.ListServerCertificatesInput, fn func(*iam.ListServerCertificatesOutput, bool) bool) error {
	fn(&m.list, true)
	return nil
}

func (m mockedIAMClient) GetServerCertificate(*iam.GetServerCertificateInput) (*iam.GetServerCertificateOutput, error) {
	return &m.cert, nil
}

func mustRead(testFile string) string {
	buf, err := os.ReadFile("testdata/" + testFile)
	if err != nil {
		panic(err)
	}
	return string(buf)
}

func TestIAM(t *testing.T) {
	foobarNotBefore := time.Date(2017, 3, 29, 16, 11, 32, 0, time.UTC)
	foobarNotAfter := time.Date(2027, 3, 27, 16, 11, 32, 0, time.UTC)
	foobarIAMCertificate := &iam.ServerCertificate{
		CertificateBody: aws.String(mustRead("foo-iam.txt")),
		ServerCertificateMetadata: &iam.ServerCertificateMetadata{
			Arn:                   aws.String("foobar-arn"),
			ServerCertificateName: aws.String("foobar"),
		},
	}

	foobarIAMCertificateWithChain := &iam.ServerCertificate{
		CertificateBody:  aws.String(mustRead("foo-iam.txt")),
		CertificateChain: aws.String(mustRead("chain.txt")),
		ServerCertificateMetadata: &iam.ServerCertificateMetadata{
			Arn:                   aws.String("foobar-arn"),
			ServerCertificateName: aws.String("foobar"),
		},
	}

	zalandoNotBefore := time.Date(2016, 3, 17, 0, 0, 0, 0, time.UTC)
	zalandoNotAfter := time.Date(2018, 3, 17, 23, 59, 59, 0, time.UTC)
	zalandoIAMCertificate := &iam.ServerCertificate{
		CertificateBody: aws.String(mustRead("zal-iam.txt")),
		ServerCertificateMetadata: &iam.ServerCertificateMetadata{
			Arn:                   aws.String("zalando-arn"),
			ServerCertificateName: aws.String("zalando"),
		},
	}

	for _, ti := range []struct {
		msg               string
		certificate       *iam.ServerCertificate
		expectARN         string
		expectNotBefore   time.Time
		expectNotAfter    time.Time
		expertDomainNames []string
	}{
		{
			msg:               "Parse foobar.de Certificate",
			certificate:       foobarIAMCertificate,
			expectARN:         "foobar-arn",
			expectNotBefore:   foobarNotBefore,
			expectNotAfter:    foobarNotAfter,
			expertDomainNames: []string{"foobar.de"},
		},
		{
			msg:               "Parse foobar.de Certificate with a Chain",
			certificate:       foobarIAMCertificateWithChain,
			expectARN:         "foobar-arn",
			expectNotBefore:   foobarNotBefore,
			expectNotAfter:    foobarNotAfter,
			expertDomainNames: []string{"foobar.de"},
		},
		{
			msg:             "Parse Zalando Certificate",
			certificate:     zalandoIAMCertificate,
			expectARN:       "zalando-arn",
			expectNotBefore: zalandoNotBefore,
			expectNotAfter:  zalandoNotAfter,
			expertDomainNames: []string{
				"fr.zalando.be",
				"fr.zalando.ch",
				"jimmy-m-fr.zalando.be",
				"jimmy-m-fr.zalando.ch",
				"jimmy-m.zalando.at",
				"jimmy-m.zalando.be",
				"jimmy-m.zalando.ch",
				"jimmy-m.zalando.co.uk",
				"jimmy-m.zalando.com",
				"jimmy-m.zalando.de",
				"jimmy-m.zalando.dk",
				"jimmy-m.zalando.es",
				"jimmy-m.zalando.fi",
				"jimmy-m.zalando.fr",
				"jimmy-m.zalando.it",
				"jimmy-m.zalando.nl",
				"jimmy-m.zalando.no",
				"jimmy-m.zalando.pl",
				"jimmy-m.zalando.se",
				"jimmy-www.zalando.at",
				"jimmy-www.zalando.be",
				"jimmy-www.zalando.ch",
				"jimmy-www.zalando.co.uk",
				"jimmy-www.zalando.com",
				"jimmy-www.zalando.de",
				"jimmy-www.zalando.dk",
				"jimmy-www.zalando.es",
				"jimmy-www.zalando.fi",
				"jimmy-www.zalando.fr",
				"jimmy-www.zalando.it",
				"jimmy-www.zalando.nl",
				"jimmy-www.zalando.no",
				"jimmy-www.zalando.pl",
				"jimmy-www.zalando.se",
				"m-fr.zalando.be",
				"m-fr.zalando.ch",
				"m.zalando.at",
				"m.zalando.be",
				"m.zalando.ch",
				"m.zalando.co.uk",
				"m.zalando.com",
				"m.zalando.de",
				"m.zalando.dk",
				"m.zalando.es",
				"m.zalando.fi",
				"m.zalando.fr",
				"m.zalando.it",
				"m.zalando.lu",
				"m.zalando.nl",
				"m.zalando.no",
				"m.zalando.pl",
				"m.zalando.se",
				"newsl-fr.zalando.be",
				"newsl-fr.zalando.ch",
				"newsl.zalando.at",
				"newsl.zalando.be",
				"newsl.zalando.ch",
				"newsl.zalando.co.uk",
				"newsl.zalando.com",
				"newsl.zalando.de",
				"newsl.zalando.dk",
				"newsl.zalando.es",
				"newsl.zalando.fi",
				"newsl.zalando.fr",
				"newsl.zalando.it",
				"newsl.zalando.lu",
				"newsl.zalando.nl",
				"newsl.zalando.no",
				"newsl.zalando.pl",
				"newsl.zalando.se",
				"www.zalando.at",
				"www.zalando.be",
				"www.zalando.ch",
				"www.zalando.co.uk",
				"www.zalando.com",
				"www.zalando.de",
				"www.zalando.dk",
				"www.zalando.es",
				"www.zalando.fi",
				"www.zalando.fr",
				"www.zalando.it",
				"www.zalando.lu",
				"www.zalando.nl",
				"www.zalando.no",
				"www.zalando.pl",
				"www.zalando.se",
			},
		},
	} {
		t.Run(ti.msg, func(t *testing.T) {
			detail, err := summaryFromServerCertificate(ti.certificate)
			require.NoError(t, err)
			require.Equal(t, ti.expectARN, detail.ID())
			require.Equal(t, ti.expertDomainNames, detail.DomainNames())
			require.Equal(t, ti.expectNotBefore, detail.NotBefore())
			require.Equal(t, ti.expectNotAfter, detail.NotAfter())
		})
	}
}

func TestIAMParseError(t *testing.T) {
	provider := iamCertificateProvider{
		api: mockedIAMClient{
			list: iam.ListServerCertificatesOutput{
				ServerCertificateMetadataList: []*iam.ServerCertificateMetadata{
					{
						Arn:                   aws.String("foobar-arn"),
						Path:                  aws.String("/"),
						ServerCertificateName: aws.String("foobar"),
					},
				},
			},
			cert: iam.GetServerCertificateOutput{
				ServerCertificate: &iam.ServerCertificate{
					CertificateBody: aws.String("..."),
				},
			},
		},
	}
	_, err := provider.GetCertificates()
	require.Equal(t, ErrNoCertificates, err)
}
