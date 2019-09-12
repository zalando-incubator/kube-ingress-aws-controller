package kubernetes

import (
	"fmt"
	"strings"
)

// ResourceLocation defines the location of Kubernetes resource in a particular
// namespace.
type ResourceLocation struct {
	Name      string
	Namespace string
}

// String implements fmt.Stringer.
func (r *ResourceLocation) String() string {
	if r == nil {
		return ""
	}

	return fmt.Sprintf("%s/%s", r.Namespace, r.Name)
}

// ParseResourceLocation parses a Kubernetes resource location from string.
// Returns an error if the string does not match the expected format of
// `namespace/name`.
func ParseResourceLocation(s string) (*ResourceLocation, error) {
	parts := strings.Split(strings.Trim(s, "/"), "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf(`invalid resource location, expected format "namespace/name" but got %q`, s)
	}

	ref := &ResourceLocation{
		Namespace: parts[0],
		Name:      parts[1],
	}

	return ref, nil
}
