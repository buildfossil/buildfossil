package capsule

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestManifestGoEnvironmentRoundTrip(t *testing.T) {
	manifest := Manifest{
		SchemaVersion: SchemaVersionV2,
		Platform: Platform{
			OS:           "darwin",
			Architecture: "arm64",
		},
		Execution: Execution{
			Argv:       []string{"go", "build", "./..."},
			WorkingDir: ".",
			ExitCode:   1,
			Stderr:     "compilation failed\n",
		},
		Runtime: &Runtime{
			Kind:    "go",
			Version: SupportedGoVersion,
		},
		GoBuildEnv: &GoBuildEnvironment{
			GOOS:       "darwin",
			GOARCH:     "arm64",
			CGOEnabled: "0",
			GOFLAGS:    "",
		},
	}

	if err := manifest.Validate(); err != nil {
		t.Fatalf("validate original: %v", err)
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
		t.Fatalf("validate decoded: %v", err)
	}

	if decoded.GoBuildEnv == nil {
		t.Fatal("Go build environment missing after JSON round-trip")
	}

	if *decoded.GoBuildEnv != *manifest.GoBuildEnv {
		t.Fatalf(
			"environment changed: got %+v, want %+v",
			decoded.GoBuildEnv,
			manifest.GoBuildEnv,
		)
	}
}

func TestManifestGoEnvironmentBackwardCompatibility(t *testing.T) {
	manifest := Manifest{
		SchemaVersion: SchemaVersionV2,
		Platform: Platform{
			OS:           "linux",
			Architecture: "amd64",
		},
		Execution: Execution{
			Argv:       []string{"/bin/sh", "-c", "exit 1"},
			WorkingDir: ".",
			ExitCode:   1,
		},
	}

	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(string(data), `"go_build_env"`) {
		t.Fatal("optional Go environment must be omitted")
	}

	var decoded Manifest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.GoBuildEnv != nil {
		t.Fatal("legacy manifest unexpectedly has Go environment")
	}

	if err := decoded.Validate(); err != nil {
		t.Fatalf("legacy manifest rejected: %v", err)
	}
}

func TestManifestRejectsInvalidGoEnvironment(t *testing.T) {
	tests := []struct {
		name    string
		runtime *Runtime
		env     GoBuildEnvironment
		wantErr string
	}{
		{
			name: "missing runtime",
			env: GoBuildEnvironment{
				GOOS: "darwin", GOARCH: "arm64", CGOEnabled: "0",
			},
			wantErr: "requires supported Go runtime",
		},
		{
			name: "invalid CGO",
			runtime: &Runtime{
				Kind: "go", Version: SupportedGoVersion,
			},
			env: GoBuildEnvironment{
				GOOS: "darwin", GOARCH: "arm64", CGOEnabled: "invalid",
			},
			wantErr: "invalid Go build environment",
		},
		{
			name: "missing architecture",
			runtime: &Runtime{
				Kind: "go", Version: SupportedGoVersion,
			},
			env: GoBuildEnvironment{
				GOOS: "darwin", CGOEnabled: "0",
			},
			wantErr: "invalid Go build environment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest := Manifest{
				SchemaVersion: SchemaVersionV2,
				Platform: Platform{
					OS: "darwin", Architecture: "arm64",
				},
				Execution: Execution{
					Argv: []string{"go", "build", "./..."},
				},
				Runtime:    tt.runtime,
				GoBuildEnv: &tt.env,
			}

			err := manifest.Validate()

			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}
