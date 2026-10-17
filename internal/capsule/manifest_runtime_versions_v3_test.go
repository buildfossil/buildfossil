package capsule

import (
	"encoding/json"
	"testing"
)

func TestManifestV3SupportedRuntimeVersions(t *testing.T) {
	tests := []struct {
		version string
		valid   bool
	}{
		{"1.26.6", true},
		{"1.25.14", true},
		{"1.26.5", false},
		{"1.25.13", false},
		{"1.25.0", false},
		{"1.27.0", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			manifest := validManifestV3()
			manifest.Runtime.Version = tt.version

			err := manifest.Validate()
			if (err == nil) != tt.valid {
				t.Fatalf(
					"Manifest.Validate version %q: err=%v, valid=%v",
					tt.version, err, tt.valid,
				)
			}

			if !tt.valid {
				return
			}

			data, err := json.Marshal(manifest)
			if err != nil {
				t.Fatal(err)
			}

			var decoded Manifest
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatal(err)
			}

			if err := decoded.Validate(); err != nil {
				t.Fatalf("decoded manifest invalid: %v", err)
			}

			if decoded.Runtime == nil ||
				decoded.Runtime.Version != tt.version {
				t.Fatalf(
					"runtime version changed in JSON round-trip: %+v",
					decoded.Runtime,
				)
			}
		})
	}
}
