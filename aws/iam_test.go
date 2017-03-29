package aws

import (
	"reflect"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/davecgh/go-spew/spew"
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

type iamExpect struct {
	List  []*CertDetail
	Error error
}

func TestIAM(t *testing.T) {
	foobarNotBefore := time.Date(2017, 3, 29, 16, 11, 32, 0, time.UTC)
	foobarNotAfter := time.Date(2027, 3, 27, 16, 11, 32, 0, time.UTC)
	foobarIAMCertificate := &iam.ServerCertificate{
		CertificateBody: aws.String(`-----BEGIN CERTIFICATE-----
MIIC+zCCAeOgAwIBAgIJANi1J+d/psHEMA0GCSqGSIb3DQEBBQUAMBQxEjAQBgNV
BAMMCWZvb2Jhci5kZTAeFw0xNzAzMjkxNjExMzJaFw0yNzAzMjcxNjExMzJaMBQx
EjAQBgNVBAMMCWZvb2Jhci5kZTCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoC
ggEBAKcAbQFpOoG11NiezABgE/TSiIXjddM8Jjxe23SXJaRHastlJvfj5IKmPe+X
+r4j8BhSe15txqb6jj8o4Whk3KaC5mU6NMprHAXcAKx8tryIuKaUicCVhlM33lIH
+kouH7QMZcixYiUah2n3rrTEBWnCp2+F4Atgd61SYNg5g23fFBUtwyFwoo1Qx44g
1vgMQR4avGlTUpsfnKZQRVimjkjr+hevepcHHpeWpsZWAyDIan+Q6bsC4Rgdm/7d
QYufeLdL76xUPilBtpvlKCfuR0XDps2ztgLOro9H0pYCADhcs9JslFOoBi9Xe3cK
DlJHRMMit2sEL13oQt/CSiGinX8CAwEAAaNQME4wHQYDVR0OBBYEFEvDk+bCv4nt
Np0wPwVeHYm38j8qMB8GA1UdIwQYMBaAFEvDk+bCv4ntNp0wPwVeHYm38j8qMAwG
A1UdEwQFMAMBAf8wDQYJKoZIhvcNAQEFBQADggEBAFiSBrSDc26JigjGQqe6N5e4
etQxAHpwzqX4mHipYOUI6iMZ++rw5py0dJa6aGBhNy4+Kr7+BeLF0FjieTxWB1Hc
xurkpI/JdeHDbz5BrSxkasf4Zyizx1zUtyo7p1AoNrQfKyBgbxjtHbgsSChEndFc
8on2d98WQlkjInTrU0qC9eXi/v35qAjWRYB+HnXD0a1Qz+/kwWmSy9YOSFJXgOml
Y2O8If2c1Rs58aKTkaOBiW+EgE4uVDP/UC69TywlArAElMO/PZi1TVkS/yCQDitc
eVXa9WiNJCPEN6bIUWZiK3Obmd+mmoVyhe9IS5QxijYrscb0tnPgFdZbPQWNBtQ=
-----END CERTIFICATE-----
`),
		ServerCertificateMetadata: &iam.ServerCertificateMetadata{
			Arn: aws.String("foobar"),
		},
	}

	zalandoNotBefore := time.Date(2016, 3, 17, 0, 0, 0, 0, time.UTC)
	zalandoNotAfter := time.Date(2018, 3, 17, 23, 59, 59, 0, time.UTC)
	zalandoIAMCertificate := &iam.ServerCertificate{
		CertificateBody: aws.String(`-----BEGIN CERTIFICATE-----
MIINdTCCDF2gAwIBAgIQV+FPccjbAdyl183GoipdpTANBgkqhkiG9w0BAQsFADCB
kjELMAkGA1UEBhMCR0IxGzAZBgNVBAgTEkdyZWF0ZXIgTWFuY2hlc3RlcjEQMA4G
A1UEBxMHU2FsZm9yZDEaMBgGA1UEChMRQ09NT0RPIENBIExpbWl0ZWQxODA2BgNV
BAMTL0NPTU9ETyBSU0EgRXh0ZW5kZWQgVmFsaWRhdGlvbiBTZWN1cmUgU2VydmVy
IENBMB4XDTE2MDMxNzAwMDAwMFoXDTE4MDMxNzIzNTk1OVowgfQxEzARBgNVBAUT
CkhSQiAxNTg4NTUxEzARBgsrBgEEAYI3PAIBAxMCREUxHTAbBgNVBA8TFFByaXZh
dGUgT3JnYW5pemF0aW9uMQswCQYDVQQGEwJERTEOMAwGA1UEERMFMTAyNDMxDzAN
BgNVBAgTBkJlcmxpbjEPMA0GA1UEBxMGQmVybGluMRMwEQYDVQQKEwpaYWxhbmRv
IFNFMTAwLgYDVQQLEydJc3N1ZWQgdGhyb3VnaCBaYWxhbmRvIFNFIEUtUEtJIE1h
bmFnZXIxIzAhBgNVBAsTGkNPTU9ETyBFViBNdWx0aS1Eb21haW4gU1NMMIIBIjAN
BgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAtWOoqGjV0VxSuaFAygLlz/w2zCBx
iOgh+68HOt5TtwzaYhe0iAAiLDgmzNUTsITtXxNEF9DtOVs4CtFUtz5ZDpEYAzUK
lMs9t5Fxh2JvG8PwQVncaXcdjIb3JIfpsN7l7WuQpZU0lD+jTiR1O/D/yMmKpo+s
WcPcCdJUh5YXk4R2YrnsuNa2MXAxLnAtobtV/HYjWGJkY/BDvIThPCqNcKVfI7T8
4ODtZxL+7pUhuITjSXh+8iJlAyI1ILxJd3TuSs8Iyl+PpJwcmsxOot1dsU3dLigl
9qP8SHt4DKpqW3419ZYbAckPSYuTYj9K+lBBuSDQBzwx0+Zk881W16/EdQIDAQAB
o4IJYTCCCV0wHwYDVR0jBBgwFoAUOdr/yigUiqh0Ewi55A6p0vp+nWkwHQYDVR0O
BBYEFNO3it1T0Lre6jovBBLY2wM3exwWMA4GA1UdDwEB/wQEAwIFoDAMBgNVHRMB
Af8EAjAAMB0GA1UdJQQWMBQGCCsGAQUFBwMBBggrBgEFBQcDAjBGBgNVHSAEPzA9
MDsGDCsGAQQBsjEBAgEFATArMCkGCCsGAQUFBwIBFh1odHRwczovL3NlY3VyZS5j
b21vZG8uY29tL0NQUzBWBgNVHR8ETzBNMEugSaBHhkVodHRwOi8vY3JsLmNvbW9k
b2NhLmNvbS9DT01PRE9SU0FFeHRlbmRlZFZhbGlkYXRpb25TZWN1cmVTZXJ2ZXJD
QS5jcmwwgYcGCCsGAQUFBwEBBHsweTBRBggrBgEFBQcwAoZFaHR0cDovL2NydC5j
b21vZG9jYS5jb20vQ09NT0RPUlNBRXh0ZW5kZWRWYWxpZGF0aW9uU2VjdXJlU2Vy
dmVyQ0EuY3J0MCQGCCsGAQUFBzABhhhodHRwOi8vb2NzcC5jb21vZG9jYS5jb20w
ggYvBgNVHREEggYmMIIGIoINZnIuemFsYW5kby5iZYINZnIuemFsYW5kby5jaIIV
amltbXktbS1mci56YWxhbmRvLmJlghVqaW1teS1tLWZyLnphbGFuZG8uY2iCEmpp
bW15LW0uemFsYW5kby5hdIISamltbXktbS56YWxhbmRvLmJlghJqaW1teS1tLnph
bGFuZG8uY2iCFWppbW15LW0uemFsYW5kby5jby51a4ITamltbXktbS56YWxhbmRv
LmNvbYISamltbXktbS56YWxhbmRvLmRlghJqaW1teS1tLnphbGFuZG8uZGuCEmpp
bW15LW0uemFsYW5kby5lc4ISamltbXktbS56YWxhbmRvLmZpghJqaW1teS1tLnph
bGFuZG8uZnKCEmppbW15LW0uemFsYW5kby5pdIISamltbXktbS56YWxhbmRvLm5s
ghJqaW1teS1tLnphbGFuZG8ubm+CEmppbW15LW0uemFsYW5kby5wbIISamltbXkt
bS56YWxhbmRvLnNlghRqaW1teS13d3cuemFsYW5kby5hdIIUamltbXktd3d3Lnph
bGFuZG8uYmWCFGppbW15LXd3dy56YWxhbmRvLmNoghdqaW1teS13d3cuemFsYW5k
by5jby51a4IVamltbXktd3d3LnphbGFuZG8uY29tghRqaW1teS13d3cuemFsYW5k
by5kZYIUamltbXktd3d3LnphbGFuZG8uZGuCFGppbW15LXd3dy56YWxhbmRvLmVz
ghRqaW1teS13d3cuemFsYW5kby5maYIUamltbXktd3d3LnphbGFuZG8uZnKCFGpp
bW15LXd3dy56YWxhbmRvLml0ghRqaW1teS13d3cuemFsYW5kby5ubIIUamltbXkt
d3d3LnphbGFuZG8ubm+CFGppbW15LXd3dy56YWxhbmRvLnBsghRqaW1teS13d3cu
emFsYW5kby5zZYIPbS1mci56YWxhbmRvLmJlgg9tLWZyLnphbGFuZG8uY2iCDG0u
emFsYW5kby5hdIIMbS56YWxhbmRvLmJlggxtLnphbGFuZG8uY2iCD20uemFsYW5k
by5jby51a4INbS56YWxhbmRvLmNvbYIMbS56YWxhbmRvLmRlggxtLnphbGFuZG8u
ZGuCDG0uemFsYW5kby5lc4IMbS56YWxhbmRvLmZpggxtLnphbGFuZG8uZnKCDG0u
emFsYW5kby5pdIIMbS56YWxhbmRvLmx1ggxtLnphbGFuZG8ubmyCDG0uemFsYW5k
by5ub4IMbS56YWxhbmRvLnBsggxtLnphbGFuZG8uc2WCE25ld3NsLWZyLnphbGFu
ZG8uYmWCE25ld3NsLWZyLnphbGFuZG8uY2iCEG5ld3NsLnphbGFuZG8uYXSCEG5l
d3NsLnphbGFuZG8uYmWCEG5ld3NsLnphbGFuZG8uY2iCE25ld3NsLnphbGFuZG8u
Y28udWuCEW5ld3NsLnphbGFuZG8uY29tghBuZXdzbC56YWxhbmRvLmRlghBuZXdz
bC56YWxhbmRvLmRrghBuZXdzbC56YWxhbmRvLmVzghBuZXdzbC56YWxhbmRvLmZp
ghBuZXdzbC56YWxhbmRvLmZyghBuZXdzbC56YWxhbmRvLml0ghBuZXdzbC56YWxh
bmRvLmx1ghBuZXdzbC56YWxhbmRvLm5sghBuZXdzbC56YWxhbmRvLm5vghBuZXdz
bC56YWxhbmRvLnBsghBuZXdzbC56YWxhbmRvLnNlgg53d3cuemFsYW5kby5hdIIO
d3d3LnphbGFuZG8uYmWCDnd3dy56YWxhbmRvLmNoghF3d3cuemFsYW5kby5jby51
a4IPd3d3LnphbGFuZG8uY29tgg53d3cuemFsYW5kby5kZYIOd3d3LnphbGFuZG8u
ZGuCDnd3dy56YWxhbmRvLmVzgg53d3cuemFsYW5kby5maYIOd3d3LnphbGFuZG8u
ZnKCDnd3dy56YWxhbmRvLml0gg53d3cuemFsYW5kby5sdYIOd3d3LnphbGFuZG8u
bmyCDnd3dy56YWxhbmRvLm5vgg53d3cuemFsYW5kby5wbIIOd3d3LnphbGFuZG8u
c2UwggF/BgorBgEEAdZ5AgQCBIIBbwSCAWsBaQB2AGj2mPgfZIK+OozuuSgdTPxx
UV1nk9RE0QpnrLtPT/vEAAABU4UWHeYAAAQDAEcwRQIhALUgp4rd9ySL1VN0E0Uk
NZP2zPmPepzn2h881ldK2ohFAiATCuLgRmBdNCLXKAmP1tbQxhAiC37v6vGM3i2P
mOEOtQB3AFYUBpov18Ls0/XhvUSyPsdGdrm8mRFcwO+UmFXWidDdAAABU4UWG7QA
AAQDAEgwRgIhAKr/ktnAwGl87W5x8zxQSLHJh5fTVhDAp4zxZ5ntBbjxAiEAp9i7
MRzRcqmwbCV+dovGop14mdB+6sYOgf3SzRZbcg4AdgCkuQmQtBhYFIe7E6LMZ3AK
PDWYBPkb37jjd80OyA3cEAAAAVOFFh3pAAAEAwBHMEUCIFB6232mZNGWKfO43MaJ
0WumNhC3KHOjrtkzN/wPzyopAiEAslUDpjAyuOsLudO9YoKPYTZa4zsTNU1TtBY1
PuE5E7wwDQYJKoZIhvcNAQELBQADggEBABOPUyYvKurP1zJUjWvmi/IzqeMP4Km9
doAxRav+B+KT/nEKAxnpifP5cxkJYGZRzs8CMRd907Nby6moBhuk5ZE1kQ5Vcs8O
ZY6bS2XTJskOBky+ZFzKnJQsfR8GLh44FuLAcW0Cp8LCUogA8HIXFnKkSoQbNvFD
G68APo3w0oRpEB+oadggPineLsBrVMuIGIALHT7bbd/3Wx3Xrrw+PqRK0IJ1M6I6
zLGcA884ssccxKw6m6lUEHsCDUh24nVbLWMWQlKdbtvIpVPMoI5PLB55NkThPzBp
4JOsEloeFmaWSMKqNq8Ra0PMhnKl3mod1kWuGj0gwNfnhPptCo6/tl4=
-----END CERTIFICATE-----`),
		ServerCertificateMetadata: &iam.ServerCertificateMetadata{
			Arn: aws.String("zalando"),
		},
	}

	for _, ti := range []struct {
		msg         string
		certificate *iam.ServerCertificate
		expect      *CertDetail
	}{
		{
			msg:         "Parse foobar.de Certificate",
			certificate: foobarIAMCertificate,
			expect: &CertDetail{
				NotAfter:  foobarNotAfter,
				NotBefore: foobarNotBefore,
				Arn:       "foobar",
				AltNames:  []string{"foobar.de"},
			},
		},
		{
			msg:         "Parse Zalando Certificate",
			certificate: zalandoIAMCertificate,
			expect: &CertDetail{
				NotAfter:  zalandoNotAfter,
				NotBefore: zalandoNotBefore,
				Arn:       "zalando",
				AltNames: []string{
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
		},
	} {
		t.Run(ti.msg, func(t *testing.T) {
			detail := certDetailFromIAM(ti.certificate)
			if !reflect.DeepEqual(*detail, *ti.expect) {
				t.Errorf("%s:\nexpected %s\ngiven: %s\n", ti.msg, spew.Sdump(ti.expect), spew.Sdump(detail))
			}
		})
	}

	for _, ti := range []struct {
		msg      string
		provider iamCertificateProvider
		expect   iamExpect
	}{
		{
			msg: "Can't parse Server Certificate",
			provider: iamCertificateProvider{
				api: mockedIAMClient{
					list: iam.ListServerCertificatesOutput{
						ServerCertificateMetadataList: []*iam.ServerCertificateMetadata{
							{
								Arn:  aws.String("foobar"),
								Path: aws.String("/"),
								ServerCertificateName: aws.String("foobar-Name"),
							},
						},
					},
					cert: iam.GetServerCertificateOutput{
						ServerCertificate: &iam.ServerCertificate{
							CertificateBody: aws.String("..."),
						},
					},
				},
			},
			expect: iamExpect{
				List:  []*CertDetail{},
				Error: nil,
			},
		},
		{
			msg: "Found iam Cert foobar",
			provider: iamCertificateProvider{
				api: mockedIAMClient{
					list: iam.ListServerCertificatesOutput{
						ServerCertificateMetadataList: []*iam.ServerCertificateMetadata{
							{
								Arn:  aws.String("foobar"),
								Path: aws.String("/"),
								ServerCertificateName: aws.String("foobar-Name"),
							},
						},
					},
					cert: iam.GetServerCertificateOutput{
						ServerCertificate: foobarIAMCertificate,
					},
				},
			},
			expect: iamExpect{
				List: []*CertDetail{
					&CertDetail{
						NotAfter:  foobarNotAfter,
						NotBefore: foobarNotBefore,
						Arn:       "foobar",
						AltNames:  []string{"foobar.de"},
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
