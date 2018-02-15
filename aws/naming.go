package aws

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

const (
	shortHashLen = 7

	maxLoadBalancerNameLen = 32
	maxStackNameLen        = 128

	nameSeparator = "-"
)

var (
	normalizationRegex = regexp.MustCompile("[^A-Za-z0-9-]+")
	squeezeDashesRegex = regexp.MustCompile("[-]{2,}")
)

// normalizeStackName normalizes the stackName by normalizing the clusterID and
// adding a uuid suffix.
func normalizeStackName(clusterID string) string {
	normalizedClusterID := squeezeDashesRegex.ReplaceAllString(
		normalizationRegex.ReplaceAllString(clusterID, nameSeparator), nameSeparator)
	lenClusterID := len(normalizedClusterID)
	maxClusterIDLen := maxStackNameLen - shortHashLen - 1
	if lenClusterID > maxClusterIDLen {
		normalizedClusterID = normalizedClusterID[lenClusterID-maxClusterIDLen:]
	}
	normalizedClusterID = strings.Trim(normalizedClusterID, nameSeparator) // trim leading/trailing separators

	return fmt.Sprintf("%s%s%s", normalizedClusterID, nameSeparator, uuid.New().String())
}
