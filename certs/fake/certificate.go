package fake

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/zalando-incubator/kube-ingress-aws-controller/certs"
)

type caSingleton struct {
	once      sync.Once
	err       error
	chainKey  *rsa.PrivateKey
	roots     *x509.CertPool
	chainCert *x509.Certificate
}

type CertificateProvider struct {
	ca caSingleton
}

func (m *CertificateProvider) GetCertificates() ([]*certs.CertificateSummary, error) {
	tenYears := time.Hour * 24 * 365 * 10
	altNames := []string{"foo.bar.org"}
	arn := "DUMMY"
	notBefore := time.Now()
	notAfter := time.Now().Add(time.Hour * 24)

	m.ca.once.Do(func() {
		caKey, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			m.ca.err = fmt.Errorf("unable to generate CA key: %v", err)
			return
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
			m.ca.err = fmt.Errorf("unable to generate CA certificate: %v", err)
			return
		}
		caReparsed, err := x509.ParseCertificate(caBody)
		if err != nil {
			m.ca.err = fmt.Errorf("unable to parse CA certificate: %v", err)
			return
		}
		m.ca.roots = x509.NewCertPool()
		m.ca.roots.AddCert(caReparsed)

		chainKey, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			m.ca.err = fmt.Errorf("unable to generate sub-CA key: %v", err)
			return
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
			m.ca.err = fmt.Errorf("unable to generate sub-CA certificate: %v", err)
		}
		chainReparsed, err := x509.ParseCertificate(chainBody)
		if err != nil {
			m.ca.err = fmt.Errorf("unable to parse sub-CA certificate: %v", err)
			return
		}

		m.ca.chainKey = chainKey
		m.ca.chainCert = chainReparsed
	})

	certKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("unable to generate certificate key: %v", err)
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

	body, err := x509.CreateCertificate(rand.Reader, &cert, m.ca.chainCert, certKey.Public(), m.ca.chainKey)
	if err != nil {
		return nil, err
	}
	reparsed, err := x509.ParseCertificate(body)
	if err != nil {
		return nil, err
	}

	c := certs.NewCertificate(arn, reparsed, []*x509.Certificate{m.ca.chainCert})
	return []*certs.CertificateSummary{c.WithRoots(m.ca.roots)}, nil
}
