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
	after := now.Add(time.Hour*24*7 + 1)

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

	// tricky times with multiple valid certs
	validCertForOneDay := createDummyCertificatDetail(validHostname, []string{}, before, now.Add(time.Hour*24))
	validCertForSixDays := createDummyCertificatDetail(validHostname, []string{}, before, now.Add(time.Hour*24*6))
	validCertForTenDays := createDummyCertificatDetail(validHostname, []string{}, before, now.Add(time.Hour*24*6))
	validCertForOneYear := createDummyCertificatDetail(validHostname, []string{}, before, now.Add(time.Hour*24*365))
	validCertSinceOneDay := createDummyCertificatDetail(validHostname, []string{}, now.Add(-time.Hour*24), after)
	validCertSinceSixDays := createDummyCertificatDetail(validHostname, []string{}, now.Add(-time.Hour*24*6), after)
	validCertSinceOneYear := createDummyCertificatDetail(validHostname, []string{}, now.Add(-time.Hour*24*365), after)
	validCertForOneYearSinceOneDay := createDummyCertificatDetail(validHostname, []string{}, now.Add(-time.Hour*24), now.Add(time.Hour*24*365))
	validCertForOneYearSinceSixDays := createDummyCertificatDetail(validHostname, []string{}, now.Add(-time.Hour*24*6), now.Add(time.Hour*24*365))
	validCertForOneYearSinceOneYear := createDummyCertificatDetail(validHostname, []string{}, now.Add(-time.Hour*24*365), now.Add(time.Hour*24*365))

	validCertFor6dUntill1y := createDummyCertificatDetail(validHostname, []string{}, now.Add(-time.Hour*24*6), now.Add(time.Hour*24*365))
	validCertFor6dUntill6d := createDummyCertificatDetail(validHostname, []string{}, now.Add(-time.Hour*24*6), now.Add(time.Hour*24*6))
	validCertFor6dUntill10d := createDummyCertificatDetail(validHostname, []string{}, now.Add(-time.Hour*24*6), now.Add(time.Hour*24*10))
	validCertFor1dUntill6d := createDummyCertificatDetail(validHostname, []string{}, now.Add(-time.Hour*24*1), now.Add(time.Hour*24*6))
	validCertFor1dUntill7d1sLess := createDummyCertificatDetail(validHostname, []string{}, now.Add(-time.Hour*24*1), now.Add(time.Hour*24*7-time.Second*1))
	validCertFor1dUntill7d1s := createDummyCertificatDetail(validHostname, []string{}, now.Add(-time.Hour*24*1), now.Add(time.Hour*24*7+time.Second*1))
	validCertFor1dUntill10d := createDummyCertificatDetail(validHostname, []string{}, now.Add(-time.Hour*24*1), now.Add(time.Hour*24*10))

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
		}, {
			msg:       "Not found best match tricky cert NotAfter 1 day compared to 6 days",
			hostname:  validHostname,
			cert:      []*acm.CertificateDetail{validCertForOneDay, validCertForSixDays},
			expect:    validCertForSixDays,
			condition: certValidMatchFunction,
		}, {
			msg:       "Not found best match tricky cert NotAfter 365 days compared to 1 day",
			hostname:  validHostname,
			cert:      []*acm.CertificateDetail{validCertForOneYear, validCertForOneDay},
			expect:    validCertForOneYear,
			condition: certValidMatchFunction,
		}, {
			msg:       "Not found best match tricky cert NotAfter 365 days compared to 6 day",
			hostname:  validHostname,
			cert:      []*acm.CertificateDetail{validCertForOneYear, validCertForSixDays},
			expect:    validCertForOneYear,
			condition: certValidMatchFunction,
		}, {
			msg:       "Not found best match tricky cert NotAfter 365 days compared to 10 day",
			hostname:  validHostname,
			cert:      []*acm.CertificateDetail{validCertForTenDays, validCertForOneYear},
			expect:    validCertForOneYear,
			condition: certValidMatchFunction,
		}, {
			msg:       "Not found best match (newest first) tricky cert NotBefore 6 days compared to 1 day",
			hostname:  validHostname,
			cert:      []*acm.CertificateDetail{validCertSinceOneDay, validCertSinceSixDays}, // FIXME: this is by order
			expect:    validCertSinceOneDay,
			condition: certValidMatchFunction,
		}, {
			msg:       "Not found best match (newest first) tricky cert NotBefore 6 days compared to 365 days",
			hostname:  validHostname,
			cert:      []*acm.CertificateDetail{validCertSinceSixDays, validCertSinceOneYear},
			expect:    validCertSinceSixDays,
			condition: certValidMatchFunction,
		}, {
			msg:       "Not found best match (newest first) tricky cert NotBefore 6 days compared to 365 days another order by cert",
			hostname:  validHostname,
			cert:      []*acm.CertificateDetail{validCertSinceOneYear, validCertSinceSixDays},
			expect:    validCertSinceSixDays,
			condition: certValidMatchFunction,
		}, {
			msg:       "Not found best match (newest first) tricky cert NotBefore 1 days compared to 365 days and both valid for 1 year",
			hostname:  validHostname,
			cert:      []*acm.CertificateDetail{validCertForOneYearSinceOneDay, validCertForOneYearSinceOneYear},
			expect:    validCertForOneYearSinceOneDay,
			condition: certValidMatchFunction,
		}, {
			msg:       "Not found best match (newest first) tricky cert NotBefore 6 days compared to 365 days and both valid for 1 year",
			hostname:  validHostname,
			cert:      []*acm.CertificateDetail{validCertForOneYearSinceOneYear, validCertForOneYearSinceSixDays},
			expect:    validCertForOneYearSinceSixDays,
			condition: certValidMatchFunction,
		}, {
			msg:       "Not found best match (newest first) tricky cert NotBefore/NotAfter 6d/1y compared to 1d/10d",
			hostname:  validHostname,
			cert:      []*acm.CertificateDetail{validCertFor6dUntill1y, validCertFor1dUntill10d},
			expect:    validCertFor1dUntill10d,
			condition: certValidMatchFunction,
		}, {
			msg:       "Not found best match (newer first) tricky cert NotBefore/NotAfter 1d/7d1s compared to 6d/10d",
			hostname:  validHostname,
			cert:      []*acm.CertificateDetail{validCertFor1dUntill7d1s, validCertFor6dUntill10d},
			expect:    validCertFor1dUntill7d1s,
			condition: certValidMatchFunction,
		}, {
			msg:       "Not found best match (longer first) tricky cert NotBefore/NotAfter 6d/6d compared to 6d/1y",
			hostname:  validHostname,
			cert:      []*acm.CertificateDetail{validCertFor6dUntill6d, validCertForOneYearSinceSixDays},
			expect:    validCertForOneYearSinceSixDays,
			condition: certValidMatchFunction,
		}, {
			msg:       "Not found best match (longer first) tricky cert NotBefore/NotAfter 1d/6d compared to 6d/10d",
			hostname:  validHostname,
			cert:      []*acm.CertificateDetail{validCertFor1dUntill6d, validCertFor6dUntill10d},
			expect:    validCertFor6dUntill10d,
			condition: certValidMatchFunction,
		}, {
			msg:       "Not found best match (longer first) tricky cert NotBefore/NotAfter 6d/10d compared to 1d/7d-1s",
			hostname:  validHostname,
			cert:      []*acm.CertificateDetail{validCertFor6dUntill10d, validCertFor1dUntill7d1sLess},
			expect:    validCertFor6dUntill10d,
			condition: certValidMatchFunction,
		},
	} {
		t.Run(ti.msg, func(t *testing.T) {
			if c, err := FindBestMatchingCertifcate(ti.cert, ti.hostname); ti.condition(err, c, ti.expect) {
				t.Errorf("%s: for host: %s expected %v, got %v, err: %v", ti.msg, ti.hostname, ti.expect, c, err)
			}

		})
	}

}

func TestGlob(t *testing.T) {
	for _, ti := range []struct {
		msg     string
		pattern string
		subj    string
		expect  bool
	}{
		{
			msg:     "Not found exact match",
			pattern: "www.foo.org",
			subj:    "www.foo.org",
			expect:  true,
		}, {
			msg:     "Not found simple glob",
			pattern: "*",
			subj:    "www.foo.org",
			expect:  true,
		}, {
			msg:     "Not found simple match",
			pattern: "*.foo.org",
			subj:    "www.foo.org",
			expect:  true,
		}, {
			msg:     "Found wrong simple match prefix",
			pattern: "www.foo.org",
			subj:    "wwww.foo.org",
			expect:  false,
		}, {
			msg:     "Found wrong simple match suffix",
			pattern: "www.foo.org",
			subj:    "www.foo.orgg",
			expect:  false,
		}, {
			msg:     "Found wrong complex match",
			pattern: "*.foo.org",
			subj:    "www.baz.foo.org",
			expect:  false,
		},
	} {
		t.Run(ti.msg, func(t *testing.T) {
			if prefixGlob(ti.pattern, ti.subj) != ti.expect {
				t.Errorf("%s: for pattern: %s and subj: %s, expected %v", ti.msg, ti.pattern, ti.subj, ti.expect)
			}

		})
	}

}
