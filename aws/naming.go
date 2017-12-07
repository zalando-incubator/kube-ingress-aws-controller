package aws

import (
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

// The Namer interface defines the behavior of types that are able to apply custom constraints to names of resources
type Namer interface {
	// Returns a normalized name from the combination of the arguments
	Normalize(clusterID, certificateARN, scheme string) string
}

// SHA1 Hash ARN, keep last 7 hex chars
// Prepend maxLen - shortHashLen - 1 chars from normalized ClusterID with a '-' as separator
// Normalization of ClusterID replaces all non valid chars
// Valid sets: a-z,A-Z,0-9,-
// Separator: -
// Squeeze and strip from beginning and/or end
type awsResourceNamer struct {
	maxLen int
}

const (
	shortHashLen = 7
	emptyARN     = 0xBADA55

	maxLoadBalancerNameLen = 32
	maxStackNameLen        = 128

	nameSeparator = "-"
)

var (
	normalizationRegex = regexp.MustCompile("[^A-Za-z0-9-]+")
	squeezeDashesRegex = regexp.MustCompile("[-]{2,}")
)

// Normalize returns a normalized name which replaces invalid characters from
// the ClusterID with '-' and appends the last 7 chars of the SHA1 hash of the
// first certificateARN. If the length of the normalized clusterID exceeds 24
// chars, the it is truncated so that its concatenation with a '-' char and the
// hash part don't exceed 32 chars.
func (n *awsResourceNamer) Normalize(clusterID string, certificateARNs []string, scheme string) string {
	hasher := sha1.New()
	if len(certificateARNs) == 0 {
		binary.Write(hasher, binary.BigEndian, emptyARN)
	} else {
		hasher.Write([]byte(certificateARNs[0]))
	}
	hash := strings.ToLower(hex.EncodeToString(hasher.Sum(nil)))
	hashLen := len(hash)
	if hashLen > shortHashLen {
		hash = hash[hashLen-shortHashLen:]
	}

	normalizedClusterID := squeezeDashesRegex.ReplaceAllString(
		normalizationRegex.ReplaceAllString(clusterID, nameSeparator), nameSeparator)
	lenClusterID := len(normalizedClusterID)
	maxClusterIDLen := n.maxLen - shortHashLen - 1
	if lenClusterID > maxClusterIDLen {
		normalizedClusterID = normalizedClusterID[lenClusterID-maxClusterIDLen:]
	}
	normalizedClusterID = strings.Trim(normalizedClusterID, nameSeparator) // trim leading/trailing separators

	return fmt.Sprintf("%s%s%s-%s", normalizedClusterID, nameSeparator, hash, scheme)
}

var (
	stackNamer = &awsResourceNamer{maxLen: maxStackNameLen}
)

func normalizeStackName(clusterID string, certificateARNs []string, scheme string) string {
	return stackNamer.Normalize(clusterID, certificateARNs, scheme)
}
