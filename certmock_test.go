package main

import "github.com/zalando-incubator/kube-ingress-aws-controller/certs"

// certmock implements CertificatesFinder for testing, without validating
// a real certificate in x509.
type certmock struct {
	summaries []*certs.CertificateSummary
}

func (m *certmock) CertificateSummaries() []*certs.CertificateSummary {
	return m.summaries
}

func (m *certmock) CertificateExists(certificateARN string) bool {
	for _, c := range m.summaries {
		if c.ID() == certificateARN {
			return true
		}
	}

	return false
}

func intersect(a, b []string) bool {
	for _, ai := range a {
		for _, bi := range b {
			if ai == bi {
				return true
			}
		}
	}

	return false
}

func (m *certmock) FindMatchingCertificateIDs(hostnames []string) []string {
	var ids []string
	for _, c := range m.summaries {
		if intersect(c.DomainNames(), hostnames) {
			ids = append(ids, c.ID())
		}
	}

	return ids
}
