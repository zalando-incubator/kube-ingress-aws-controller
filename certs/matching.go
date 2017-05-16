package certs

import (
	"errors"
	"strings"
	"time"
)

const (
	// minimal time period for the NotAfter attribute of a Cert to be in the future
	minimalCertValidityPeriod = 7 * 24 * time.Hour
	// used as wildcard char in Cert Hostname/AltName matches
	glob = "*"
)

// ErrNoMatchingCertificateFound is used if there is no matching ACM certificate found
var ErrNoMatchingCertificateFound = errors.New("no matching certificate found")

// FindBestMatchingCertificate uses a suffix search, best match operation, in order to find the best matching
// certificate for a given hostname.
func FindBestMatchingCertificate(certs []*CertificateSummary, hostname string) (*CertificateSummary, error) {
	candidate := &CertificateSummary{}
	longestMatch := -1
	now := time.Now()

	for _, cert := range certs {
		notAfter := cert.NotAfter()
		notBefore := cert.NotBefore()

		// ignore invalid timeframes
		if !cert.IsValidAt(now) {
			continue
		}

		for _, altName := range cert.SubjectAlternativeNames() {
			if prefixGlob(altName, hostname) {
				nameLength := len(altName)

				switch {
				case longestMatch < 0:
					// first matching found
					longestMatch = nameLength
					candidate = cert
				case longestMatch < nameLength:
					if notBefore.Before(now) && notAfter.Add(-minimalCertValidityPeriod).After(now) {
						// more specific valid cert found: *.example.org -> foo.example.org
						longestMatch = nameLength
						candidate = cert
					}
				case longestMatch == nameLength:
					if notBefore.After(candidate.NotBefore()) &&
						!notAfter.Add(-minimalCertValidityPeriod).Before(now) {
						// cert is newer than curBestCert and is not invalid in 7 days
						longestMatch = nameLength
						candidate = cert
					} else if notBefore.Equal(candidate.NotBefore()) && !candidate.NotAfter().After(notAfter) {
						// cert has the same issue date, but is longer valid
						longestMatch = nameLength
						candidate = cert
					} else if notBefore.Before(candidate.NotBefore()) &&
						candidate.NotAfter().Add(-minimalCertValidityPeriod).Before(now) &&
						notAfter.After(candidate.NotAfter()) {
						// cert is older than curBestCert but curBestCert is invalid in 7 days and cert is longer valid
						longestMatch = nameLength
						candidate = cert
					}
				case longestMatch > nameLength:
					if candidate.NotAfter().Add(-minimalCertValidityPeriod).Before(now) &&
						now.Before(candidate.NotBefore()) &&
						notBefore.Before(now) &&
						now.Before(notAfter.Add(-minimalCertValidityPeriod)) {
						// foo.example.org -> *.example.org degradation when NotAfter requires a downgrade
						longestMatch = nameLength
						candidate = cert
					}
				}
			}
		}
	}

	if longestMatch == -1 {
		return nil, ErrNoMatchingCertificateFound
	}
	return candidate, nil
}

func prefixGlob(pattern, subj string) bool {
	// Empty pattern can only match empty subject
	if pattern == "" {
		return subj == pattern
	}

	// If the pattern _is_ a glob, it matches everything
	if pattern == glob {
		return true
	}

	leadingGlob := strings.HasPrefix(pattern, glob)

	if !leadingGlob {
		// No globs in pattern, so test for equality
		return subj == pattern
	}

	pat := string(pattern[1:])
	trimmedSubj := strings.TrimSuffix(subj, pat)
	return !strings.Contains(trimmedSubj, ".")
}
