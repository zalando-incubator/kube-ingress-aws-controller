package certs

import (
	"crypto/x509"
	"time"
)

// CertificatesProvider interface for Certificate Provider like local,
// AWS IAM or AWS ACM
type CertificatesProvider interface {
	GetCertificates() ([]*CertificateSummary, error)
}

// CertificateSummary is the business object for Certificates
type CertificateSummary struct {
	id          string
	certificate *x509.Certificate
	chain       *x509.CertPool
	roots       *x509.CertPool
	domainNames []string
}

// NewCertificate returns a new CertificateSummary with the matching
// fields set from the arguments
func NewCertificate(id string, certificate *x509.Certificate, chain []*x509.Certificate) *CertificateSummary {
	var domainNames []string

	if certificate.Subject.CommonName != "" {
		domainNames = append(domainNames, certificate.Subject.CommonName)
	}
	domainNames = append(domainNames, certificate.DNSNames...)

	chainPool := x509.NewCertPool()
	for _, chainCert := range chain {
		chainPool.AddCert(chainCert)
	}

	return &CertificateSummary{
		id:          id,
		certificate: certificate,
		chain:       chainPool,
		domainNames: domainNames,
	}
}

// WithRoots enables you to override the root certificate pool.
// This should be only used for test purposes.
func (c *CertificateSummary) WithRoots(r *x509.CertPool) *CertificateSummary {
	c.roots = r
	return c
}

// ID returns the certificate ID for the underlying provider
func (c *CertificateSummary) ID() string {
	return c.id
}

// DomainNames returns all the host names
// (sites, IP addresses, common names, etc.) protected by the
// certificate
func (c *CertificateSummary) DomainNames() []string {
	return c.domainNames
}

// NotBefore returns the field with the same name from the certificate
func (c *CertificateSummary) NotBefore() time.Time {
	return c.certificate.NotBefore
}

// NotAfter returns the field with the same name from the certificate
func (c *CertificateSummary) NotAfter() time.Time {
	return c.certificate.NotAfter
}

// Verify attempts to verify the certificate against the roots
// using the chain information if needed, for TLS usage.
func (c *CertificateSummary) Verify(hostname string) error {
	opts := x509.VerifyOptions{
		DNSName:       hostname,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		Roots:         c.roots,
		Intermediates: c.chain,
		CurrentTime:   currentTime(),
	}
	_, err := c.certificate.Verify(opts)
	return err
}

// For tests: allow overriding current time
var currentTime = time.Now
