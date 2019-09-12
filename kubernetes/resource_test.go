package kubernetes

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseResourceLocation(t *testing.T) {
	for _, test := range []struct {
		name          string
		s             string
		expected      *ResourceLocation
		expectedError string
	}{
		{
			name:          "empty string",
			expectedError: `invalid resource location, expected format "namespace/name" but got ""`,
		},
		{
			name:          "missing namespace",
			s:             "/foo-name",
			expectedError: `invalid resource location, expected format "namespace/name" but got "/foo-name"`,
		},
		{
			name:          "missing name",
			s:             "foo-ns/",
			expectedError: `invalid resource location, expected format "namespace/name" but got "foo-ns/"`,
		},
		{
			name:     "valid resource location",
			s:        "foo-ns/foo-name",
			expected: &ResourceLocation{Namespace: "foo-ns", Name: "foo-name"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			result, err := ParseResourceLocation(test.s)
			if test.expectedError != "" {
				require.Error(t, err)
				assert.Equal(t, test.expectedError, err.Error())
			} else {
				require.NoError(t, err)
				assert.Equal(t, test.expected, result)
			}
		})
	}
}
