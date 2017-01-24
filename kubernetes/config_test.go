package kubernetes

import (
	"fmt"
	"os"
	"testing"
)

func TestInClusterConfig(t *testing.T) {
	for _, test := range []struct {
		hostEnvVar             string
		portEnvVar             string
		serviceAccountLocation string
		wantBaseURL            string
		wantError              bool
	}{
		{"localhost", "8001", "testdata/", "https://localhost:8001", false},
		{"", "8001", "testdata/", "https://localhost:8001", true},
		{"localhost", "", "testdata/", "https://localhost:8001", true},
		{"localhost", "8001", "testdata/missing/", "https://localhost:8001", true},
		{"localhost", "8001", "testdata/missingca/", "https://localhost:8001", true},
		{"", "", "", "https://localhost:8001", true},
	} {
		t.Run(fmt.Sprintf("%v/%v/%v", test.hostEnvVar, test.portEnvVar, test.serviceAccountLocation), func(t *testing.T) {
			os.Setenv(serviceHostEnvVar, test.hostEnvVar)
			os.Setenv(servicePortEnvVar, test.portEnvVar)
			serviceAccountLocator = func() string {
				return test.serviceAccountLocation
			}
			cfg, err := InClusterConfig()
			if test.wantError {
				if err == nil {
					t.Error("expected an error")
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if cfg.BaseURL != test.wantBaseURL {
					t.Errorf(`unexpected base URL. wanted %q, got %q`, test.wantBaseURL, cfg.BaseURL)
				}
			}
		})
	}
}

func TestWellKnownServiceAccountLocation(t *testing.T) {
	loc := defaultServiceAccountLocator()
	if loc != serviceAccountDir {
		t.Errorf("unexpected service account location. wanted %q, got %q", serviceAccountDir, loc)
	}
}
