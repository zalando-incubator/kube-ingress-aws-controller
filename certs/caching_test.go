package certs

import (
	"context"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockCertificateProvider struct {
	mu       sync.Mutex
	certs    []*CertificateSummary
	err      error
	numCalls int
}

func (m *mockCertificateProvider) GetCertificates(ctx context.Context) ([]*CertificateSummary, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.numCalls++
	return m.certs, m.err
}

func (m *mockCertificateProvider) setResult(certs []*CertificateSummary, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.certs = certs
	m.err = err
}

func (m *mockCertificateProvider) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.numCalls
}

func newTestCertificateSummary(t *testing.T, id string) *CertificateSummary {
	t.Helper()
	caCert := x509.Certificate{
		SerialNumber: big.NewInt(123),
		Subject: pkix.Name{
			Organization: []string{"Testing CA"},
		},
		NotBefore:             time.Time{},
		NotAfter:              time.Now().Add(tenYears),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	return NewCertificate(id, &caCert, nil)
}

func TestNewCachingProviderInitialLoadFailure(t *testing.T) {
	ctx := context.Background()
	provider := &mockCertificateProvider{err: errors.New("boom")}

	_, err := NewCachingProvider(ctx, time.Minute*10, map[string]bool{}, provider)

	require.Error(t, err)
	require.Contains(t, err.Error(), "initial load of certificates failed")
	require.Contains(t, err.Error(), "boom")
}

func TestNewCachingProviderBlacklistFiltering(t *testing.T) {
	ctx := context.Background()
	allowed := newTestCertificateSummary(t, "allowed")
	blacklisted := newTestCertificateSummary(t, "blacklisted")
	provider := &mockCertificateProvider{certs: []*CertificateSummary{allowed, blacklisted}}

	cachingProvider, err := NewCachingProvider(ctx, time.Minute*10, map[string]bool{"blacklisted": true}, provider)
	require.NoError(t, err)

	summaries, err := cachingProvider.GetCertificates(ctx)
	require.NoError(t, err)
	require.Len(t, summaries, 1)
	assert.Equal(t, "allowed", summaries[0].ID())
}

func TestNewCachingProviderMultipleProviders(t *testing.T) {
	ctx := context.Background()
	providerA := &mockCertificateProvider{certs: []*CertificateSummary{newTestCertificateSummary(t, "a")}}
	providerB := &mockCertificateProvider{certs: []*CertificateSummary{newTestCertificateSummary(t, "b")}}

	cachingProvider, err := NewCachingProvider(ctx, time.Minute*10, map[string]bool{}, providerA, providerB)
	require.NoError(t, err)

	summaries, err := cachingProvider.GetCertificates(ctx)
	require.NoError(t, err)
	require.Len(t, summaries, 2)

	ids := []string{summaries[0].ID(), summaries[1].ID()}
	assert.ElementsMatch(t, []string{"a", "b"}, ids)
}

func TestBackgroundRefreshUpdatesCache(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	provider := &mockCertificateProvider{certs: []*CertificateSummary{newTestCertificateSummary(t, "initial")}}

	cachingProvider, err := NewCachingProvider(ctx, 5*time.Millisecond, map[string]bool{}, provider)
	require.NoError(t, err)

	provider.setResult([]*CertificateSummary{newTestCertificateSummary(t, "refreshed")}, nil)

	require.Eventually(t, func() bool {
		summaries, err := cachingProvider.GetCertificates(ctx)
		return err == nil && len(summaries) == 1 && summaries[0].ID() == "refreshed"
	}, time.Second, 5*time.Millisecond, "expected background refresh to update the cache")
}

func TestBackgroundRefreshFailureKeepsLastKnownGoodCache(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	provider := &mockCertificateProvider{certs: []*CertificateSummary{newTestCertificateSummary(t, "initial")}}

	cachingProvider, err := NewCachingProvider(ctx, 5*time.Millisecond, map[string]bool{}, provider)
	require.NoError(t, err)

	provider.setResult(nil, errors.New("refresh failed"))

	require.Eventually(t, func() bool {
		return provider.callCount() >= 2
	}, time.Second, 5*time.Millisecond, "expected background refresh to have been attempted again")

	summaries, err := cachingProvider.GetCertificates(ctx)
	require.NoError(t, err)
	require.Len(t, summaries, 1)
	assert.Equal(t, "initial", summaries[0].ID())
}

func TestBackgroundRefreshStopsWhenContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	provider := &mockCertificateProvider{certs: []*CertificateSummary{newTestCertificateSummary(t, "initial")}}

	_, err := NewCachingProvider(ctx, 5*time.Millisecond, map[string]bool{}, provider)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return provider.callCount() >= 1
	}, time.Second, 5*time.Millisecond, "expected at least one background refresh before cancellation")

	cancel()
	callsAtCancel := provider.callCount()

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, callsAtCancel, provider.callCount(), "expected no further refreshes after context cancellation")
}

func TestCachingProvider(t *testing.T) {
	ctx := context.Background()
	var (
		blacklistCertArnMap = make(map[string]bool)
	)
	cachingProvider, err := NewCachingProvider(
		ctx,
		time.Minute*10,
		blacklistCertArnMap,
		&mockCertificateProvider{certs: []*CertificateSummary{newTestCertificateSummary(t, "foo")}},
	)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	summaries, err := cachingProvider.GetCertificates(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("Expected length of 1, got: %v", len(summaries))
	}
	certificateSummary := summaries[0]
	if certificateSummary.ID() != "foo" {
		t.Fatalf("Expected foo, got: %v", certificateSummary.ID())
	}
}
