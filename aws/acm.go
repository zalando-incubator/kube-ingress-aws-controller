package aws

import (
	"log"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/acm"
	"github.com/aws/aws-sdk-go/service/acm/acmiface"
)

// CertDetail is the business object for Certificates
type CertDetail struct {
	Arn       string
	AltNames  []string
	NotBefore time.Time
	NotAfter  time.Time
}

func newCertDetail(acmDetail *acm.CertificateDetail) *CertDetail {
	var altNames []string
	for _, alt := range acmDetail.SubjectAlternativeNames {
		altNames = append(altNames, aws.StringValue(alt))
	}
	return &CertDetail{
		Arn:       aws.StringValue(acmDetail.CertificateArn),
		AltNames:  append(altNames, aws.StringValue(acmDetail.DomainName)),
		NotBefore: aws.TimeValue(acmDetail.NotBefore),
		NotAfter:  aws.TimeValue(acmDetail.NotAfter),
	}
}

const (
	// minimal timeperiod for the NotAfter attribute of a Cert to be in the future
	minimalCertValidityPeriod = 7 * 24 * time.Hour
	// used as wildcard char in Cert Hostname/AltName matches
	glob = "*"
)

type certificateCache struct {
	sync.Mutex
	acmClient      acmiface.ACMAPI
	acmCertSummary []*acm.CertificateSummary
	acmCertDetail  []*acm.CertificateDetail
}

func newCertCache(acmClient acmiface.ACMAPI) *certificateCache {
	return &certificateCache{
		acmClient:      acmClient,
		acmCertSummary: make([]*acm.CertificateSummary, 0),
		acmCertDetail:  make([]*acm.CertificateDetail, 0),
	}
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

// updateCertCache will only update the current certificateCache if
// all calls to AWS API were successfull and not rateLimited for
// example. In case it could not update the certificateCache it will
// return the orginal error.
func (cc *certificateCache) updateCertCache() error {
	certList, err := cc.listCerts()
	if err != nil {
		return err
	}

	certDetails := make([]*acm.CertificateDetail, len(certList), len(certList))
	for idx, o := range certList {
		certInput := &acm.DescribeCertificateInput{CertificateArn: o.CertificateArn}
		certDetail, err := cc.acmClient.DescribeCertificate(certInput)
		if err != nil {
			return err
		}
		certDetails[idx] = certDetail.Certificate
	}

	cc.Lock()
	cc.acmCertSummary = certList
	cc.acmCertDetail = certDetails
	cc.Unlock()
	return nil
}

// GetCachedCerts returns a copy of the cached acm CertificateDetail slice
// https://docs.aws.amazon.com/acm/latest/APIReference/API_ListCertificates.html#API_ListCertificates_RequestSyntax
// filtered by CertificateStatuses
// https://docs.aws.amazon.com/acm/latest/APIReference/API_ListCertificates.html#API_ListCertificates_RequestSyntax
func (cc *certificateCache) GetCachedCerts() []*CertDetail {
	cc.Lock()
	copy := cc.acmCertDetail[:]
	cc.Unlock()
	result := make([]*CertDetail, 0, 1)
	for _, c := range copy {
		result = append(result, newCertDetail(c))
	}

	return result
}

// LsitCerts returns a list of acm Certificates filtered by
// CertificateStatuses
// https://docs.aws.amazon.com/acm/latest/APIReference/API_ListCertificates.html#API_ListCertificates_RequestSyntax
func (cc *certificateCache) listCerts() ([]*acm.CertificateSummary, error) {
	certList := make([]*acm.CertificateSummary, 0)
	params := &acm.ListCertificatesInput{
		CertificateStatuses: []*string{
			aws.String("ISSUED"), // Required
		},
	}
	err := cc.acmClient.ListCertificatesPages(params, func(page *acm.ListCertificatesOutput, lastPage bool) bool {
		for _, cert := range page.CertificateSummaryList {
			certList = append(certList, cert)
		}
		return true // never stop iterator
	})

	return certList, err
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
