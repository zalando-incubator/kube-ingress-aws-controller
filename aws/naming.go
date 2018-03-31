package aws

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

const (
	maxStackNameLen = 128
	uuidLen         = 36
	nameSeparator   = "-"
	stackNamePrefix = "ingress-alb"
)

var (
	normalizationRegex = regexp.MustCompile("[^A-Za-z0-9-]+")
	squeezeDashesRegex = regexp.MustCompile("[-]{2,}")
)

// normalizeStackName normalizes the stackName by normalizing the clusterID,
// adding a stack name prefix and a uuid suffix.
func normalizeStackName(clusterID string) string {
	normalizedClusterID := squeezeDashesRegex.ReplaceAllString(
		normalizationRegex.ReplaceAllString(clusterID, nameSeparator), nameSeparator)
	lenClusterID := len(normalizedClusterID)
	// max cluser ID length is the max stack name length except stack name
	// prefix, UUID and two separators.
	maxClusterIDLen := maxStackNameLen - len(stackNamePrefix) - uuidLen - 2
	if lenClusterID > maxClusterIDLen {
		normalizedClusterID = normalizedClusterID[lenClusterID-maxClusterIDLen:]
	}
	normalizedClusterID = strings.Trim(normalizedClusterID, nameSeparator) // trim leading/trailing separators

	return fmt.Sprintf("%s%s%s%s%s", stackNamePrefix, nameSeparator, normalizedClusterID, nameSeparator, uuid.New().String())
}
