package certs

import "time"

// CertificatesProvider interface for Certificate Provider like local,
// AWS IAM or AWS ACM
type CertificatesProvider interface {
	GetCertificates() ([]*CertificateSummary, error)
}

// CertificateSummary is the business object for Certificates
type CertificateSummary struct {
	id        string
	san       []string
	notBefore time.Time
	notAfter  time.Time
}

// NewCertificate returns a new CertificateSummary with the matching
// fields set from the arguments
func NewCertificate(id string, san []string, notBefore time.Time, notAfter time.Time) *CertificateSummary {
	return &CertificateSummary{
		id:        id,
		san:       san,
		notBefore: notBefore,
		notAfter:  notAfter,
	}
}

// ID returns the certificate ID for the underlying provider
func (c *CertificateSummary) ID() string {
	return c.id
}

// SubjectAlternativeNames returns all the additional host names
// (sites, IP addresses, common names, etc.) protected by the
// certificate
func (c *CertificateSummary) SubjectAlternativeNames() []string {
	return c.san
}

// NotBefore returns the field with the same name from the certificate
func (c *CertificateSummary) NotBefore() time.Time {
	return c.notBefore
}

// NotAfter returns the field with the same name from the certificate
func (c *CertificateSummary) NotAfter() time.Time {
	return c.notAfter
}

// IsValidAt asserts if the the argument is contained in the
// certificate's date interval
func (c *CertificateSummary) IsValidAt(when time.Time) bool {
	return when.Before(c.notAfter) && when.After(c.notBefore)
}
