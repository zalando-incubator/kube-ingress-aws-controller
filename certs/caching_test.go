package certs

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"testing"
	"time"
)

type mockedCertificateProvider struct {
}

func (m mockedCertificateProvider) GetCertificates() ([]*CertificateSummary, error) {
	caCert := x509.Certificate{
		SerialNumber: big.NewInt(123),
		Subject: pkix.Name{
			Organization: []string{"Testing CA"},
		},
		NotBefore: time.Time{},
		NotAfter:  time.Now().Add(tenYears),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,

		IsCA: true,
	}
	certificateSummary := NewCertificate(
		"foo",
		&caCert,
		nil)
	return []*CertificateSummary{certificateSummary}, nil
}

func TestCachingProvider(t *testing.T) {
	var (
		blacklistCertArnMap = make(map[string]bool)
	)
	cachingProvider, err := NewCachingProvider(
		time.Minute*10,
		blacklistCertArnMap,
		mockedCertificateProvider{},
	)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	summaries, err := cachingProvider.GetCertificates()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("Expected length of 1, got: %v", len(summaries))
	}
	certificateSummary := summaries[0]
	if certificateSummary.ID() != "foo" {
		t.Fatalf("Expected foo, got: %v", certificateSummary.ID())
	}
}
