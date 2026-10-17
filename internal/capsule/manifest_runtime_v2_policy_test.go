package capsule

import "testing"

func TestManifestV2PreservesOriginalGoRuntimePolicy(t *testing.T) {
	tests := []struct {
		name    string
		version string
		valid   bool
	}{
		{"original supported version", "1.26.6", true},
		{"new v3-only version", "1.25.14", false},
		{"unsupported patch", "1.25.13", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Manifest{
				SchemaVersion: SchemaVersionV2,
				Execution: Execution{
					Argv:       []string{"go", "build", "./..."},
					WorkingDir: ".",
					ExitCode:   1,
				},
				Platform: Platform{
					OS: "darwin", Architecture: "arm64",
				},
				Runtime: &Runtime{
					Kind: "go", Version: tt.version,
				},
				GoBuildEnv: &GoBuildEnvironment{
					GOOS: "linux", GOARCH: "amd64",
					CGOEnabled: "0",
				},
			}

			err := m.Validate()
			if (err == nil) != tt.valid {
				t.Fatalf(
					"version %q: err=%v, valid=%v",
					tt.version, err, tt.valid,
				)
			}
		})
	}
}
