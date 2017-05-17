package certs

import (
	"testing"
	"time"
)

func createDummyCertDetail(arn, domainName string, altNames []string, notBefore, notAfter time.Time) *CertificateSummary {
	return NewCertificate(arn, append(altNames, domainName), notBefore, notAfter)
}

type certCondition func(error, *CertificateSummary, *CertificateSummary) bool

func certValidMatchFunction(err error, expect, c *CertificateSummary) bool {
	return err != nil || c != expect
}

func certInvalidMatchFunction(err error, expect, c *CertificateSummary) bool {
	return err == nil && c == expect
}

func TestFindBestMatchingCertificate(t *testing.T) {
	domain := "example.org"
	wildcardDomain := "*." + domain
	invalidDomain := "invalid.org"
	invalidWildcardDomain := "*." + invalidDomain
	validHostname := "foo." + domain
	invalidHostname := "foo." + invalidDomain

	now := time.Now()
	before := now.Add(-time.Hour * 24 * 7)
	after := now.Add(time.Hour*24*7 + 1*time.Second)
	dummyArn := "DUMMY"

	// simple cert
	validCert := createDummyCertDetail(dummyArn, validHostname, []string{}, before, after)
	validWildcardCert := createDummyCertDetail(dummyArn, wildcardDomain, []string{}, before, after)
	invalidDomainCert := createDummyCertDetail(dummyArn, invalidDomain, []string{}, before, after)
	invalidWildcardCert := createDummyCertDetail(dummyArn, invalidWildcardDomain, []string{}, before, after)

	// AlternateName certs
	saSingleValidCert := createDummyCertDetail(dummyArn, "", []string{validHostname}, before, after)
	saCnWrongValidCert := createDummyCertDetail(dummyArn, invalidHostname, []string{validHostname}, before, after)
	saValidCert := createDummyCertDetail(dummyArn, "", []string{validHostname, invalidDomain, invalidHostname, invalidWildcardDomain}, before, after)
	saValidWildcardCert := createDummyCertDetail(dummyArn, "", []string{invalidDomain, invalidHostname, invalidWildcardDomain, wildcardDomain}, before, after)
	saMultipleValidCert := createDummyCertDetail(dummyArn, "", []string{wildcardDomain, validHostname, invalidDomain, invalidHostname, invalidWildcardDomain}, before, after)

	// simple invalid time cases
	invalidTimeCert1 := createDummyCertDetail(dummyArn, domain, []string{}, after, before)
	invalidTimeCert2 := createDummyCertDetail(dummyArn, domain, []string{}, after, after)
	invalidTimeCert3 := createDummyCertDetail(dummyArn, domain, []string{}, before, before)

	// tricky times with multiple valid certs
	validCertForOneDay := createDummyCertDetail(dummyArn, validHostname, []string{}, before, now.Add(time.Hour*24))
	validCertForSixDays := createDummyCertDetail(dummyArn, validHostname, []string{}, before, now.Add(time.Hour*24*6))
	validCertForTenDays := createDummyCertDetail(dummyArn, validHostname, []string{}, before, now.Add(time.Hour*24*6))
	validCertForOneYear := createDummyCertDetail(dummyArn, validHostname, []string{}, before, now.Add(time.Hour*24*365))
	validCertSinceOneDay := createDummyCertDetail(dummyArn, validHostname, []string{}, now.Add(-time.Hour*24), after)
	validCertSinceSixDays := createDummyCertDetail(dummyArn, validHostname, []string{}, now.Add(-time.Hour*24*6), after)
	validCertSinceOneYear := createDummyCertDetail(dummyArn, validHostname, []string{}, now.Add(-time.Hour*24*365), after)
	validCertForOneYearSinceOneDay := createDummyCertDetail(dummyArn, validHostname, []string{}, now.Add(-time.Hour*24), now.Add(time.Hour*24*365))
	validCertForOneYearSinceSixDays := createDummyCertDetail(dummyArn, validHostname, []string{}, now.Add(-time.Hour*24*6), now.Add(time.Hour*24*365))
	validCertForOneYearSinceOneYear := createDummyCertDetail(dummyArn, validHostname, []string{}, now.Add(-time.Hour*24*365), now.Add(time.Hour*24*365))

	validCertFor6dUntill1y := createDummyCertDetail(dummyArn, validHostname, []string{}, now.Add(-time.Hour*24*6), now.Add(time.Hour*24*365))
	validCertFor6dUntill6d := createDummyCertDetail(dummyArn, validHostname, []string{}, now.Add(-time.Hour*24*6), now.Add(time.Hour*24*6))
	validCertFor6dUntill10d := createDummyCertDetail(dummyArn, validHostname, []string{}, now.Add(-time.Hour*24*6), now.Add(time.Hour*24*10))
	validCertFor1dUntill6d := createDummyCertDetail(dummyArn, validHostname, []string{}, now.Add(-time.Hour*24*1), now.Add(time.Hour*24*6))
	validCertFor1dUntill7d1sLess := createDummyCertDetail(dummyArn, validHostname, []string{}, now.Add(-time.Hour*24*1), now.Add(time.Hour*24*7-time.Second*1))
	validCertFor1dUntill7d1s := createDummyCertDetail(dummyArn, validHostname, []string{}, now.Add(-time.Hour*24*1), now.Add(time.Hour*24*7+time.Second*1))
	validCertFor1dUntill10d := createDummyCertDetail(dummyArn, validHostname, []string{}, now.Add(-time.Hour*24*1), now.Add(time.Hour*24*10))

	for _, ti := range []struct {
		msg       string
		hostname  string
		cert      []*CertificateSummary
		expect    *CertificateSummary
		condition certCondition
	}{
		{
			msg:       "Not found best match",
			hostname:  validHostname,
			cert:      []*CertificateSummary{validCert},
			expect:    validCert,
			condition: certValidMatchFunction,
		}, {
			msg:       "Not found wildcard as best match",
			hostname:  validHostname,
			cert:      []*CertificateSummary{validWildcardCert},
			expect:    validWildcardCert,
			condition: certValidMatchFunction,
		}, {
			msg:       "Not found best match of multiple valid certs",
			hostname:  validHostname,
			cert:      []*CertificateSummary{validCert, validWildcardCert},
			expect:    validCert,
			condition: certValidMatchFunction,
		}, {
			msg:       "Not found best match of multiple certs one wildcard valid",
			hostname:  validHostname,
			cert:      []*CertificateSummary{invalidDomainCert, validWildcardCert, invalidWildcardCert},
			expect:    validWildcardCert,
			condition: certValidMatchFunction,
		}, {
			msg:       "Not found best match of multiple certs one valid",
			hostname:  validHostname,
			cert:      []*CertificateSummary{invalidDomainCert, validCert, invalidWildcardCert},
			expect:    validCert,
			condition: certValidMatchFunction,
		}, {
			msg:       "Found best match for invalid hostname",
			hostname:  invalidHostname,
			cert:      []*CertificateSummary{validCert},
			expect:    nil,
			condition: certInvalidMatchFunction,
		}, {
			msg:       "Found best match for invalid cert",
			hostname:  validHostname,
			cert:      []*CertificateSummary{invalidDomainCert},
			expect:    nil,
			condition: certInvalidMatchFunction,
		}, {
			msg:       "Found best match for invalid wildcardcert",
			hostname:  validHostname,
			cert:      []*CertificateSummary{invalidWildcardCert},
			expect:    nil,
			condition: certInvalidMatchFunction,
		}, {
			msg:       "Found best match for multiple invalid certs",
			hostname:  validHostname,
			cert:      []*CertificateSummary{invalidWildcardCert, invalidDomainCert},
			expect:    nil,
			condition: certInvalidMatchFunction,
		}, {
			msg:       "Not found best match of AlternateName cert",
			hostname:  validHostname,
			cert:      []*CertificateSummary{saSingleValidCert},
			expect:    saSingleValidCert,
			condition: certValidMatchFunction,
		}, {
			msg:       "Not found best match of AlternateName cert with wrong Cn",
			hostname:  validHostname,
			cert:      []*CertificateSummary{saCnWrongValidCert},
			expect:    saCnWrongValidCert,
			condition: certValidMatchFunction,
		}, {
			msg:       "Not found best match of AlternateName cert with one valid and multiple invalid names",
			hostname:  validHostname,
			cert:      []*CertificateSummary{saValidCert},
			expect:    saValidCert,
			condition: certValidMatchFunction,
		}, {
			msg:       "Not found best match of AlternateName cert with one valid wildcard and multiple invalid names",
			hostname:  validHostname,
			cert:      []*CertificateSummary{saValidWildcardCert},
			expect:    saValidWildcardCert,
			condition: certValidMatchFunction,
		}, {
			msg:       "Not found best match of AlternateName cert with multiple valid and multiple invalid names",
			hostname:  validHostname,
			cert:      []*CertificateSummary{saMultipleValidCert},
			expect:    saMultipleValidCert,
			condition: certValidMatchFunction,
		}, {
			msg:       "Found best match for invalid time cert 1",
			hostname:  validHostname,
			cert:      []*CertificateSummary{invalidTimeCert1},
			expect:    nil,
			condition: certInvalidMatchFunction,
		}, {
			msg:       "Found best match for invalid time cert 2",
			hostname:  validHostname,
			cert:      []*CertificateSummary{invalidTimeCert2},
			expect:    nil,
			condition: certInvalidMatchFunction,
		}, {
			msg:       "Found best match for invalid time cert 3",
			hostname:  validHostname,
			cert:      []*CertificateSummary{invalidTimeCert3},
			expect:    nil,
			condition: certInvalidMatchFunction,
		}, {
			msg:       "Not found best match tricky cert NotAfter 1 day compared to 6 days",
			hostname:  validHostname,
			cert:      []*CertificateSummary{validCertForOneDay, validCertForSixDays},
			expect:    validCertForSixDays,
			condition: certValidMatchFunction,
		}, {
			msg:       "Not found best match tricky cert NotAfter 365 days compared to 1 day",
			hostname:  validHostname,
			cert:      []*CertificateSummary{validCertForOneYear, validCertForOneDay},
			expect:    validCertForOneYear,
			condition: certValidMatchFunction,
		}, {
			msg:       "Not found best match tricky cert NotAfter 365 days compared to 6 day",
			hostname:  validHostname,
			cert:      []*CertificateSummary{validCertForOneYear, validCertForSixDays},
			expect:    validCertForOneYear,
			condition: certValidMatchFunction,
		}, {
			msg:       "Not found best match tricky cert NotAfter 365 days compared to 10 day",
			hostname:  validHostname,
			cert:      []*CertificateSummary{validCertForTenDays, validCertForOneYear},
			expect:    validCertForOneYear,
			condition: certValidMatchFunction,
		}, {
			msg:       "Not found best match (newest first) tricky cert NotBefore 6 days compared to 1 day",
			hostname:  validHostname,
			cert:      []*CertificateSummary{validCertSinceOneDay, validCertSinceSixDays}, // FIXME: this is by order
			expect:    validCertSinceOneDay,
			condition: certValidMatchFunction,
		}, {
			msg:       "Not found best match (newest first) tricky cert NotBefore 6 days compared to 365 days",
			hostname:  validHostname,
			cert:      []*CertificateSummary{validCertSinceSixDays, validCertSinceOneYear},
			expect:    validCertSinceSixDays,
			condition: certValidMatchFunction,
		}, {
			msg:       "Not found best match (newest first) tricky cert NotBefore 6 days compared to 365 days another order by cert",
			hostname:  validHostname,
			cert:      []*CertificateSummary{validCertSinceOneYear, validCertSinceSixDays},
			expect:    validCertSinceSixDays,
			condition: certValidMatchFunction,
		}, {
			msg:       "Not found best match (newest first) tricky cert NotBefore 1 days compared to 365 days and both valid for 1 year",
			hostname:  validHostname,
			cert:      []*CertificateSummary{validCertForOneYearSinceOneDay, validCertForOneYearSinceOneYear},
			expect:    validCertForOneYearSinceOneDay,
			condition: certValidMatchFunction,
		}, {
			msg:       "Not found best match (newest first) tricky cert NotBefore 6 days compared to 365 days and both valid for 1 year",
			hostname:  validHostname,
			cert:      []*CertificateSummary{validCertForOneYearSinceOneYear, validCertForOneYearSinceSixDays},
			expect:    validCertForOneYearSinceSixDays,
			condition: certValidMatchFunction,
		}, {
			msg:       "Not found best match (newest first) tricky cert NotBefore/NotAfter 6d/1y compared to 1d/10d",
			hostname:  validHostname,
			cert:      []*CertificateSummary{validCertFor6dUntill1y, validCertFor1dUntill10d},
			expect:    validCertFor1dUntill10d,
			condition: certValidMatchFunction,
		}, {
			msg:       "Not found best match (newer first) tricky cert NotBefore/NotAfter 1d/7d1s compared to 6d/10d",
			hostname:  validHostname,
			cert:      []*CertificateSummary{validCertFor1dUntill7d1s, validCertFor6dUntill10d},
			expect:    validCertFor1dUntill7d1s,
			condition: certValidMatchFunction,
		}, {
			msg:       "Not found best match (longer first) tricky cert NotBefore/NotAfter 6d/6d compared to 6d/1y",
			hostname:  validHostname,
			cert:      []*CertificateSummary{validCertFor6dUntill6d, validCertForOneYearSinceSixDays},
			expect:    validCertForOneYearSinceSixDays,
			condition: certValidMatchFunction,
		}, {
			msg:       "Not found best match (longer first) tricky cert NotBefore/NotAfter 1d/6d compared to 6d/10d",
			hostname:  validHostname,
			cert:      []*CertificateSummary{validCertFor1dUntill6d, validCertFor6dUntill10d},
			expect:    validCertFor6dUntill10d,
			condition: certValidMatchFunction,
		}, {
			msg:       "Not found best match (longer first) tricky cert NotBefore/NotAfter 6d/10d compared to 1d/7d-1s",
			hostname:  validHostname,
			cert:      []*CertificateSummary{validCertFor6dUntill10d, validCertFor1dUntill7d1sLess},
			expect:    validCertFor6dUntill10d,
			condition: certValidMatchFunction,
		},
	} {
		t.Run(ti.msg, func(t *testing.T) {
			if c, err := FindBestMatchingCertificate(ti.cert, ti.hostname); ti.condition(err, c, ti.expect) {
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
