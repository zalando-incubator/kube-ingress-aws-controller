package aws

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/acm"
)

func createDummyCertificatDetail(domainName string, altNames []string, notBefore, notAfter time.Time) *acm.CertificateDetail {
	var alt []*string
	for _, v := range altNames {
		alt = append(alt, aws.String(v))
	}

	return &acm.CertificateDetail{
		DomainName:              aws.String(domainName),
		NotAfter:                aws.Time(notAfter),
		NotBefore:               aws.Time(notBefore),
		SubjectAlternativeNames: alt,
	}
}

type certCondition func(error, *acm.CertificateDetail, *acm.CertificateDetail) bool

func certValidMatchFunction(err error, expect, c *acm.CertificateDetail) bool {
	return err != nil || c != expect
}

func certInvalidMatchFunction(err error, expect, c *acm.CertificateDetail) bool {
	return err == nil && c == expect
}

func TestFindBestMatchingCertifcate(t *testing.T) {
	domain := "example.org"
	wildcardDomain := "*." + domain
	invalidDomain := "invalid.org"
	invalidWildcardDomain := "*." + invalidDomain
	validHostname := "foo." + domain
	invalidHostname := "foo." + invalidDomain

	now := time.Now()
	before := now.Add(-time.Hour * 24 * 7)
	after := now.Add(time.Hour * 24 * 7)

	// simple cert
	validCert := createDummyCertificatDetail(validHostname, []string{}, before, after)
	validWildcardCert := createDummyCertificatDetail(wildcardDomain, []string{}, before, after)
	invalidDomainCert := createDummyCertificatDetail(invalidDomain, []string{}, before, after)
	invalidWildcardCert := createDummyCertificatDetail(invalidWildcardDomain, []string{}, before, after)

	// AlternateName certs
	saSingleValidCert := createDummyCertificatDetail("", []string{validHostname}, before, after)
	saCnWrongValidCert := createDummyCertificatDetail(invalidHostname, []string{validHostname}, before, after)
	saValidCert := createDummyCertificatDetail("", []string{validHostname, invalidDomain, invalidHostname, invalidWildcardDomain}, before, after)
	saValidWildcardCert := createDummyCertificatDetail("", []string{invalidDomain, invalidHostname, invalidWildcardDomain, wildcardDomain}, before, after)
	saMultipleValidCert := createDummyCertificatDetail("", []string{wildcardDomain, validHostname, invalidDomain, invalidHostname, invalidWildcardDomain}, before, after)

	// simple invalid time cases
	invalidTimeCert1 := createDummyCertificatDetail(domain, []string{}, after, before)
	invalidTimeCert2 := createDummyCertificatDetail(domain, []string{}, after, after)
	invalidTimeCert3 := createDummyCertificatDetail(domain, []string{}, before, before)

	for _, ti := range []struct {
		msg       string
		hostname  string
		cert      []*acm.CertificateDetail
		expect    *acm.CertificateDetail
		condition certCondition
	}{
		{
			msg:       "Not found best match",
			hostname:  validHostname,
			cert:      []*acm.CertificateDetail{validCert},
			expect:    validCert,
			condition: certValidMatchFunction,
		}, {
			msg:       "Not found wildcard as best match",
			hostname:  validHostname,
			cert:      []*acm.CertificateDetail{validWildcardCert},
			expect:    validWildcardCert,
			condition: certValidMatchFunction,
		}, {
			msg:       "Not found best match of multiple valid certs",
			hostname:  validHostname,
			cert:      []*acm.CertificateDetail{validCert, validWildcardCert},
			expect:    validCert,
			condition: certValidMatchFunction,
		}, {
			msg:       "Not found best match of multiple certs one wildcard valid",
			hostname:  validHostname,
			cert:      []*acm.CertificateDetail{invalidDomainCert, validWildcardCert, invalidWildcardCert},
			expect:    validWildcardCert,
			condition: certValidMatchFunction,
		}, {
			msg:       "Not found best match of multiple certs one valid",
			hostname:  validHostname,
			cert:      []*acm.CertificateDetail{invalidDomainCert, validCert, invalidWildcardCert},
			expect:    validCert,
			condition: certValidMatchFunction,
		}, {
			msg:       "Found best match for invalid hostname",
			hostname:  invalidHostname,
			cert:      []*acm.CertificateDetail{validCert},
			expect:    nil,
			condition: certInvalidMatchFunction,
		}, {
			msg:       "Found best match for invalid cert",
			hostname:  validHostname,
			cert:      []*acm.CertificateDetail{invalidDomainCert},
			expect:    nil,
			condition: certInvalidMatchFunction,
		}, {
			msg:       "Found best match for invalid wildcardcert",
			hostname:  validHostname,
			cert:      []*acm.CertificateDetail{invalidWildcardCert},
			expect:    nil,
			condition: certInvalidMatchFunction,
		}, {
			msg:       "Found best match for multiple invalid certs",
			hostname:  validHostname,
			cert:      []*acm.CertificateDetail{invalidWildcardCert, invalidDomainCert},
			expect:    nil,
			condition: certInvalidMatchFunction,
		}, {
			msg:       "Not found best match of AlternateName cert",
			hostname:  validHostname,
			cert:      []*acm.CertificateDetail{saSingleValidCert},
			expect:    saSingleValidCert,
			condition: certValidMatchFunction,
		}, {
			msg:       "Not found best match of AlternateName cert with wrong Cn",
			hostname:  validHostname,
			cert:      []*acm.CertificateDetail{saCnWrongValidCert},
			expect:    saCnWrongValidCert,
			condition: certValidMatchFunction,
		}, {
			msg:       "Not found best match of AlternateName cert with one valid and multiple invalid names",
			hostname:  validHostname,
			cert:      []*acm.CertificateDetail{saValidCert},
			expect:    saValidCert,
			condition: certValidMatchFunction,
		}, {
			msg:       "Not found best match of AlternateName cert with one valid wildcard and multiple invalid names",
			hostname:  validHostname,
			cert:      []*acm.CertificateDetail{saValidWildcardCert},
			expect:    saValidWildcardCert,
			condition: certValidMatchFunction,
		}, {
			msg:       "Not found best match of AlternateName cert with multiple valid and multiple invalid names",
			hostname:  validHostname,
			cert:      []*acm.CertificateDetail{saMultipleValidCert},
			expect:    saMultipleValidCert,
			condition: certValidMatchFunction,
		}, {
			msg:       "Found best match for invalid time cert 1",
			hostname:  validHostname,
			cert:      []*acm.CertificateDetail{invalidTimeCert1},
			expect:    nil,
			condition: certInvalidMatchFunction,
		}, {
			msg:       "Found best match for invalid time cert 2",
			hostname:  validHostname,
			cert:      []*acm.CertificateDetail{invalidTimeCert2},
			expect:    nil,
			condition: certInvalidMatchFunction,
		}, {
			msg:       "Found best match for invalid time cert 3",
			hostname:  validHostname,
			cert:      []*acm.CertificateDetail{invalidTimeCert3},
			expect:    nil,
			condition: certInvalidMatchFunction,
		},
	} {
		t.Run(ti.msg, func(t *testing.T) {
			if c, err := FindBestMatchingCertifcate(ti.cert, ti.hostname); ti.condition(err, c, ti.expect) {
				t.Errorf("%s: for host: %s expected %v, got %v, err: %v", ti.msg, ti.hostname, ti.expect, c, err)
			}

		})
	}

}
