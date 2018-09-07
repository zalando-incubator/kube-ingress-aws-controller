package aws

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
)

var (
	// ErrNoCertificates is used to signal that no certificates were found in the PEM data
	ErrNoCertificates = errors.New("no certificates found in PEM data")

	// ErrTooManyCertificates is used to signal that multiple certificates were found in the PEM data where we expect only one
	ErrTooManyCertificates = errors.New("too many certificates found in PEM data")
)

// ParseCertificates parses X509 PEM-encoded certificates from a string
func ParseCertificates(pemCertificates string) ([]*x509.Certificate, error) {
	var result []*x509.Certificate

	bytes := []byte(pemCertificates)
	for {
		block, rest := pem.Decode([]byte(bytes))
		if block == nil {
			return result, nil
		}

		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, err
		}

		result = append(result, cert)
		bytes = rest
	}
}

// ParseCertificate parses exactly one X509 PEM-encoded certificate from a string
func ParseCertificate(pemCertificate string) (*x509.Certificate, error) {
	certs, err := ParseCertificates(pemCertificate)
	if err != nil {
		return nil, err
	}
	if len(certs) == 0 {
		return nil, ErrNoCertificates
	}
	if len(certs) > 1 {
		return nil, ErrTooManyCertificates
	}
	return certs[0], nil
}
