package fake

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"time"

	"github.com/zalando-incubator/kube-ingress-aws-controller/certs"
)

type CA struct {
	chainKey  *rsa.PrivateKey
	roots     *x509.CertPool
	chainCert *x509.Certificate
}

func NewCA() (*CA, error) {
	const tenYears = time.Hour * 24 * 365 * 10

	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("unable to generate CA key: %w", err)
	}

	caCert := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Testing CA"},
		},
		NotBefore: time.Time{},
		NotAfter:  time.Now().Add(tenYears),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,

		IsCA: true,
	}

	caBody, err := x509.CreateCertificate(rand.Reader, &caCert, &caCert, caKey.Public(), caKey)
	if err != nil {
		return nil, fmt.Errorf("unable to generate CA certificate: %w", err)
	}

	caReparsed, err := x509.ParseCertificate(caBody)
	if err != nil {
		return nil, fmt.Errorf("unable to parse CA certificate: %w", err)
	}

	chainKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("unable to generate sub-CA key: %w", err)
	}
	chainCert := x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			Organization: []string{"Testing Sub-CA"},
		},
		NotBefore: time.Time{},
		NotAfter:  time.Now().Add(tenYears),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,

		IsCA: true,
	}

	chainBody, err := x509.CreateCertificate(rand.Reader, &chainCert, caReparsed, chainKey.Public(), caKey)
	if err != nil {
		return nil, fmt.Errorf("unable to generate sub-CA certificate: %w", err)
	}

	chainReparsed, err := x509.ParseCertificate(chainBody)
	if err != nil {
		return nil, fmt.Errorf("unable to parse sub-CA certificate: %w", err)
	}

	ca := new(CA)
	ca.roots = x509.NewCertPool()
	ca.roots.AddCert(caReparsed)
	ca.chainKey = chainKey
	ca.chainCert = chainReparsed

	return ca, nil
}

func (ca *CA) NewCertificateSummary() (*certs.CertificateSummary, error) {
	altNames := []string{"foo.bar.org"}
	arn := "DUMMY"
	notBefore := time.Now()
	notAfter := time.Now().Add(time.Hour * 24)

	certKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("unable to generate certificate key: %w", err)
	}

	cert := x509.Certificate{
		SerialNumber: big.NewInt(3),
		DNSNames:     altNames,
		NotBefore:    notBefore,
		NotAfter:     notAfter,

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	body, err := x509.CreateCertificate(rand.Reader, &cert, ca.chainCert, certKey.Public(), ca.chainKey)
	if err != nil {
		return nil, err
	}

	reparsed, err := x509.ParseCertificate(body)
	if err != nil {
		return nil, err
	}

	c := certs.NewCertificate(arn, reparsed, []*x509.Certificate{ca.chainCert})
	return c.WithRoots(ca.roots), nil
}

type CertificateProvider struct {
	Summaries []*certs.CertificateSummary
	Error     error
}

func (m *CertificateProvider) GetCertificates(_ context.Context) ([]*certs.CertificateSummary, error) {
	return m.Summaries, m.Error
}

// certmock implements CertificatesFinder for testing, without validating
// a real certificate in x509.
type Cert struct {
	summaries []*certs.CertificateSummary
}

func NewCert(summaries []*certs.CertificateSummary) *Cert {
	return &Cert{
		summaries: summaries,
	}
}

func (m *Cert) CertificateSummaries() []*certs.CertificateSummary {
	return m.summaries
}

func (m *Cert) CertificateExists(certificateARN string) bool {
	for _, c := range m.summaries {
		if c.ID() == certificateARN {
			return true
		}
	}

	return false
}

func intersect(a, b []string) bool {
	for _, ai := range a {
		for _, bi := range b {
			if ai == bi {
				return true
			}
		}
	}

	return false
}

func (m *Cert) FindMatchingCertificateIDs(hostnames []string) []string {
	var ids []string
	for _, c := range m.summaries {
		if intersect(c.DomainNames(), hostnames) {
			ids = append(ids, c.ID())
		}
	}

	return ids
}
