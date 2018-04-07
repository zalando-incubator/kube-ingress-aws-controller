package aws

import (
	"strings"
	"testing"
)

func TestNormalizeStackName(t *testing.T) {
	// test simple cluster ID
	clusterID := "my-cluster"
	normalized := normalizeStackName(clusterID)
	expectedPrefix := stackNamePrefix + nameSeparator + clusterID + nameSeparator
	if !strings.HasPrefix(normalized, expectedPrefix) {
		t.Errorf("expected prefix %s, got %s", expectedPrefix, normalized)
	}

	// test that very long cluster ID gets cut off
	longClusterID := strings.Repeat("a", maxStackNameLen)
	normalized = normalizeStackName(longClusterID)
	expectedClusterID := strings.Repeat("a", maxStackNameLen-len(stackNamePrefix)-uuidLen-2)
	expectedPrefix = stackNamePrefix + nameSeparator + expectedClusterID + nameSeparator
	if !strings.HasPrefix(normalized, expectedPrefix) {
		t.Errorf("expected prefix %s, got %s", expectedPrefix, normalized)
	}
}
