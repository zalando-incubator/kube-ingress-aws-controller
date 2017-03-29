package aws

import (
	"log"
	"strings"
	"sync"
	"time"
)

const (
	// minimal timeperiod for the NotAfter attribute of a Cert to be in the future
	minimalCertValidityPeriod = 7 * 24 * time.Hour
	// used as wildcard char in Cert Hostname/AltName matches
	glob = "*"
)

// CertDetail is the business object for Certificates
type CertDetail struct {
	Arn       string
	AltNames  []string
	NotBefore time.Time
	NotAfter  time.Time
}

// CertificatesProvider interface for Certificate Provider like local, AWS IAM or AWS ACM
type CertificatesProvider interface {
	GetCertificates() ([]*CertDetail, error)
}

type certificateCache struct {
	sync.Mutex
	providers   []CertificatesProvider
	certDetails []*CertDetail
}

func newCertCache(providers ...CertificatesProvider) *certificateCache {
	return &certificateCache{
		providers:   providers,
		certDetails: make([]*CertDetail, 0),
	}
}

type certProviderWrapper struct {
	certs []*CertDetail
	err   error
}

// updateCertCache will only update the current certificateCache if
// all calls to AWS API were successfull and not rateLimited for
// example. In case it could not update the certificateCache it will
// return the orginal error.
func (cc *certificateCache) updateCertCache() error {
	var wg sync.WaitGroup
	ch := make(chan certProviderWrapper, len(cc.providers))
	wg.Add(len(cc.providers))
	for _, cp := range cc.providers {
		go func(provider CertificatesProvider) {
			res, err := provider.GetCertificates()
			ch <- certProviderWrapper{certs: res, err: err}
			wg.Done()
		}(cp)
	}
	wg.Wait()
	close(ch)
	newList := make([]*CertDetail, 0)
	for providerResponse := range ch {
		if providerResponse.err != nil {
			return providerResponse.err
		}
		newList = append(newList, providerResponse.certs...)
	}
	cc.Lock()
	cc.certDetails = newList
	cc.Unlock()
	return nil
}

func (cc *certificateCache) backgroundCertCacheUpdate(certUpdateInterval time.Duration) {
	go func() {
		for {
			time.Sleep(certUpdateInterval)
			if err := cc.updateCertCache(); err != nil {
				log.Printf("Cert cache update failed, caused by: %v", err)
			} else {
				log.Println("Successfully updated cert cache")
			}
		}
	}()
}

// GetCachedCerts returns a copy of the cached CertDetails
func (cc *certificateCache) GetCachedCerts() []*CertDetail {
	cc.Lock()
	copy := cc.certDetails[:]
	cc.Unlock()

	return copy
}

// FindBestMatchingCertificate get all ACM certificates and use a suffix
// search best match operation in order to find the best matching
// certificate ARN.
//
// We don't need to validate the Revocation here, because we only pull
// ISSUED certificates.
func FindBestMatchingCertificate(certs []*CertDetail, hostname string) (*CertDetail, error) {
	candidate := &CertDetail{}
	longestMatch := -1
	now := time.Now()

	for _, cert := range certs {
		notAfter := cert.NotAfter
		notBefore := cert.NotBefore

		// ignore invalid timeframes
		if now.After(notAfter) || now.Before(notBefore) {
			continue
		}

		for _, altName := range cert.AltNames {
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
					if notBefore.After(candidate.NotBefore) &&
						!notAfter.Add(-minimalCertValidityPeriod).Before(now) {
						// cert is newer than curBestCert and is not invalid in 7 days
						longestMatch = nameLength
						candidate = cert
					} else if notBefore.Equal(candidate.NotBefore) && !candidate.NotAfter.After(notAfter) {
						// cert has the same issue date, but is longer valid
						longestMatch = nameLength
						candidate = cert
					} else if notBefore.Before(candidate.NotBefore) &&
						candidate.NotAfter.Add(-minimalCertValidityPeriod).Before(now) &&
						notAfter.After(candidate.NotAfter) {
						// cert is older than curBestCert but curBestCert is invalid in 7 days and cert is longer valid
						longestMatch = nameLength
						candidate = cert
					}
				case longestMatch > nameLength:
					if candidate.NotAfter.Add(-minimalCertValidityPeriod).Before(now) &&
						now.Before(candidate.NotBefore) &&
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
