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

type certificateCache struct {
	sync.Mutex
	acmClient      acmiface.ACMAPI
	acmCertSummary []*acm.CertificateSummary
	acmCertDetail  []*acm.CertificateDetail
}

func NewCertCache(acmClient acmiface.ACMAPI) *certificateCache {
	return &certificateCache{
		acmClient:      acmClient,
		acmCertSummary: make([]*acm.CertificateSummary, 0),
		acmCertDetail:  make([]*acm.CertificateDetail, 0),
	}
}

func (cc *certificateCache) InitCertCache(certUpdateInterval time.Duration) {
	go func() {
		for {
			log.Println("update cert cache")
			cc.updateCertCache()
			time.Sleep(certUpdateInterval)
		}
	}()

}

// TODO(sszuecs): make it successfull when we get AWS rateLimited
func (cc *certificateCache) updateCertCache() {
	certList, err := cc.GetCerts()
	if err != nil {
		log.Printf("Could not update certificate cache, caused by: %v", err)
		return
	}

	certDetails := make([]*acm.CertificateDetail, len(certList), len(certList))
	for idx, o := range certList {
		certInput := &acm.DescribeCertificateInput{CertificateArn: o.CertificateArn}
		certDetail, err := cc.acmClient.DescribeCertificate(certInput)
		if err != nil {
			log.Printf("Could not get certificate details from AWS for ARN: %s, caused by: %v", o.CertificateArn, err)
			return
		}
		certDetails[idx] = certDetail.Certificate
	}

	cc.Lock()
	cc.acmCertSummary = certList
	cc.acmCertDetail = certDetails
	cc.Unlock()
}

// GetCachedCerts returns a copy of the cached acm Certifcates
// filtered by CertificateStatuses
// https://docs.aws.amazon.com/acm/latest/APIReference/API_ListCertificates.html#API_ListCertificates_RequestSyntax
// no locking is required to access certs.
func (cc *certificateCache) GetCachedCerts() []*acm.CertificateDetail {
	cc.Lock()
	result := cc.acmCertDetail[:]
	cc.Unlock()
	return result
}

// GetCerts returns a list of acm Certifcates filtered by
// CertificateStatuses
// https://docs.aws.amazon.com/acm/latest/APIReference/API_ListCertificates.html#API_ListCertificates_RequestSyntax
func (cc *certificateCache) GetCerts() ([]*acm.CertificateSummary, error) {
	maxItems := aws.Int64(10)

	params := &acm.ListCertificatesInput{
		CertificateStatuses: []*string{
			aws.String("ISSUED"), // Required
		},
		MaxItems: maxItems,
	}
	resp, err := cc.acmClient.ListCertificates(params)
	if err != nil {
		return nil, err
	}
	certList := resp.CertificateSummaryList

	// more certs if NextToken set in response, use pagination to get more
	for resp.NextToken != nil {
		params = &acm.ListCertificatesInput{
			CertificateStatuses: []*string{
				aws.String("ISSUED"),
			},
			MaxItems:  maxItems,
			NextToken: resp.NextToken,
		}
		resp, err = cc.acmClient.ListCertificates(params)
		if err != nil {
			return nil, err
		}

		for _, cert := range resp.CertificateSummaryList {
			certList = append(certList, cert)
		}
	}

	return certList, nil
}

// FindBestMatchingCertifcate get all ACM certifcates and use a suffix
// search best match opertion in order to find the best matching
// certifcate ARN.
//
// We don't need to validate the Revocation here, because we only pull
// ISSUED certificates.
func FindBestMatchingCertifcate(certs []*acm.CertificateDetail, hostname string) (*acm.CertificateDetail, error) {
	candidates := map[int]*acm.CertificateDetail{}
	longestGlob := -1

	for _, cert := range certs {
		// ignore invalid timeframes
		if cert.NotAfter.Unix() <= time.Now().Unix() || time.Now().Unix() <= cert.NotBefore.Unix() {
			continue
		}
		altNames := append(cert.SubjectAlternativeNames, cert.DomainName)
		for _, altName := range altNames {
			if Glob(aws.StringValue(altName), hostname) {
				l := len(aws.StringValue(altName))
				if longestGlob < l {
					longestGlob = l
					candidates[l] = cert
				} else if longestGlob == l {
					// TODO: check cert details https://github.bus.zalan.do/teapot/issues/issues/315

				}
			}
		}
	}

	if longestGlob == -1 {
		return nil, ErrNoMatchingCertificateFound
	}
	return candidates[longestGlob], nil
}

// modified version of https://github.com/ryanuber/go-glob/blob/master/glob.go (MIT licensed)
func Glob(pattern, subj string) bool {
	// Empty pattern can only match empty subject
	if pattern == "" {
		return subj == pattern
	}

	// If the pattern _is_ a glob, it matches everything
	if pattern == GLOB {
		return true
	}

	parts := strings.Split(pattern, GLOB)

	if len(parts) == 1 {
		// No globs in pattern, so test for equality
		return subj == pattern
	}

	leadingGlob := strings.HasPrefix(pattern, GLOB)
	end := len(parts) - 1

	// Go over the leading parts and ensure they match.
	for i := 0; i < end; i++ {
		idx := strings.Index(subj, parts[i])

		switch i {
		case 0:
			// Check the first section. Requires special handling.
			if !leadingGlob && idx != 0 {
				return false
			}
		default:
			// Check that the middle parts match.
			if idx < 0 {
				return false
			}
		}

		// Trim evaluated text from subj as we loop over the pattern.
		subj = subj[idx+len(parts[i]):]
	}

	// Reached the last section. Requires special handling.
	return strings.HasSuffix(subj, parts[end])
}
