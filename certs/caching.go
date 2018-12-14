package certs

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"sync"
	"time"
)

type cachingProvider struct {
	sync.Mutex
	providers         []CertificatesProvider
	certDetails       []*CertificateSummary
	blacklistedArnMap map[string]bool
}

type certProviderWrapper struct {
	certs []*CertificateSummary
	err   error
}

// NewCachingProvider collects certificates from multiple providers
// and keeps them cached in memory.  After an initial loading of
// certificates it will continue to refresh the cache every
// certUpdateInterval in the background. If the background refresh
// fails the last known cached values are considered current.
func NewCachingProvider(certUpdateInterval time.Duration, blacklistedArnMap map[string]bool, providers ...CertificatesProvider) (CertificatesProvider, error) {
	provider := &cachingProvider{
		providers:         providers,
		blacklistedArnMap: blacklistedArnMap,
		certDetails:       make([]*CertificateSummary, 0),
	}
	if err := provider.updateCertCache(); err != nil {
		return nil, fmt.Errorf("initial load of certificates failed: %v", err)
	}
	provider.startBackgroundRefresh(certUpdateInterval)
	return provider, nil
}

// GetCertificates returns a copy of the cached certificates
func (cc *cachingProvider) GetCertificates() ([]*CertificateSummary, error) {
	cc.Lock()
	certCopy := cc.certDetails[:]
	cc.Unlock()

	return certCopy, nil
}

// updateCertCache will only update the current certificate cache if
// all providers are successful.  In case it fails it will return the
// original error.
func (cc *cachingProvider) updateCertCache() error {
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
	newList := make([]*CertificateSummary, 0)
	for providerResponse := range ch {
		if providerResponse.err != nil {
			return providerResponse.err
		}

		provisionCerts := make([]*CertificateSummary, 0)
		for _, certSummary := range providerResponse.certs {
			if _, ok := cc.blacklistedArnMap[certSummary.ID()]; !ok {
				provisionCerts = append(provisionCerts, certSummary)
			}
		}

		newList = append(newList, provisionCerts...)
	}
	cc.Lock()
	cc.certDetails = newList
	cc.Unlock()
	return nil
}

// startBackgroundRefresh creates a background forever loop to update
// certificate cache.
func (cc *cachingProvider) startBackgroundRefresh(certUpdateInterval time.Duration) {
	go func() {
		for {
			time.Sleep(certUpdateInterval)
			if err := cc.updateCertCache(); err != nil {
				log.Infof("certificate cache background update failed: %v", err)
			}
		}
	}()
}
