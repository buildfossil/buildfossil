package replay

import (
	"testing"

	"github.com/rewtools/rew/internal/capsule"
)

func TestGoRuntimeVersionPolicyBySchema(t *testing.T) {
	tests := []struct {
		name     string
		schema   int
		version  string
		accepted bool
	}{
		{"v2 original", capsule.SchemaVersionV2, "1.26.6", true},
		{"v2 new version rejected", capsule.SchemaVersionV2, "1.25.14", false},
		{"v3 original", capsule.SchemaVersionV3, "1.26.6", true},
		{"v3 new version", capsule.SchemaVersionV3, "1.25.14", true},
		{"v3 unknown version", capsule.SchemaVersionV3, "1.25.13", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := capsule.Manifest{
				SchemaVersion: tt.schema,
				Platform: capsule.Platform{
					OS: "darwin", Architecture: "arm64",
				},
				Runtime: &capsule.Runtime{
					Kind: "go", Version: tt.version,
				},
				GoBuildEnv: &capsule.GoBuildEnvironment{
					GOOS: "linux", GOARCH: "amd64",
					CGOEnabled: "0",
				},
			}

			err := validatePortableGoReplay(m)
			if (err == nil) != tt.accepted {
				t.Fatalf("portability: err=%v, accepted=%v", err, tt.accepted)
			}

			if got := shouldProbeGoDependencies(m); got != tt.accepted {
				t.Fatalf(
					"dependency probe = %v, want %v",
					got, tt.accepted,
				)
			}
		})
	}
}
