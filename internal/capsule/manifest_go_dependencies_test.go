package capsule

import (
	"encoding/json"
	"testing"
)

func TestManifestGoDependenciesJSON(t *testing.T) {
	manifest := Manifest{
		SchemaVersion: SchemaVersionV2,
		Runtime: &Runtime{
			Kind:    "go",
			Version: SupportedGoVersion,
		},
		GoDependencies: &GoDependencyStatus{
			State: GoDependenciesUnknown,
		},
	}

	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}

	var decoded Manifest

	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.GoDependencies == nil {
		t.Fatal("GoDependencies missing after JSON round trip")
	}

	if decoded.GoDependencies.State != GoDependenciesUnknown {
		t.Fatalf(
			"state = %q, want %q",
			decoded.GoDependencies.State,
			GoDependenciesUnknown,
		)
	}
}

func TestManifestGoDependenciesOmitted(t *testing.T) {
	manifest := Manifest{
		SchemaVersion: SchemaVersionV2,
	}

	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}

	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if _, exists := decoded["go_dependencies"]; exists {
		t.Fatal("go_dependencies must be omitted when nil")
	}
}

func TestManifestGoDependenciesValidation(t *testing.T) {
	tests := []struct {
		name         string
		schema       int
		runtime      *Runtime
		dependencies *GoDependencyStatus
		wantErr      bool
	}{
		{
			name:   "valid Go status",
			schema: SchemaVersionV2,
			runtime: &Runtime{
				Kind:    "go",
				Version: SupportedGoVersion,
			},
			dependencies: &GoDependencyStatus{
				State: GoDependenciesUnknown,
			},
		},
		{
			name:   "reject schema v1",
			schema: SchemaVersionV1,
			runtime: &Runtime{
				Kind:    "go",
				Version: SupportedGoVersion,
			},
			dependencies: &GoDependencyStatus{
				State: GoDependenciesReady,
			},
			wantErr: true,
		},
		{
			name:   "reject missing runtime",
			schema: SchemaVersionV2,
			dependencies: &GoDependencyStatus{
				State: GoDependenciesReady,
			},
			wantErr: true,
		},
		{
			name:   "reject unsupported runtime",
			schema: SchemaVersionV2,
			runtime: &Runtime{
				Kind:    "go",
				Version: "0.0.0",
			},
			dependencies: &GoDependencyStatus{
				State: GoDependenciesReady,
			},
			wantErr: true,
		},
		{
			name:   "reject invalid state",
			schema: SchemaVersionV2,
			runtime: &Runtime{
				Kind:    "go",
				Version: SupportedGoVersion,
			},
			dependencies: &GoDependencyStatus{
				State: "invalid",
			},
			wantErr: true,
		},
		{
			name:   "reject unavailable without reason",
			schema: SchemaVersionV2,
			runtime: &Runtime{
				Kind:    "go",
				Version: SupportedGoVersion,
			},
			dependencies: &GoDependencyStatus{
				State: GoDependenciesUnavailable,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest := Manifest{
				SchemaVersion: tt.schema,
				Execution: Execution{
					Argv:     []string{"go", "build", "./..."},
					ExitCode: 1,
					Stderr:   "build failed\n",
				},
				Platform: Platform{
					OS:           "linux",
					Architecture: "amd64",
				},
				Runtime:        tt.runtime,
				GoDependencies: tt.dependencies,
			}

			err := manifest.Validate()

			if (err != nil) != tt.wantErr {
				t.Fatalf(
					"Validate() error = %v, wantErr = %v",
					err,
					tt.wantErr,
				)
			}
		})
	}
}

func TestManifestLegacyV2WithoutGoDependencies(t *testing.T) {
	manifest := Manifest{
		SchemaVersion: SchemaVersionV2,
		Execution: Execution{
			Argv:     []string{"go", "build", "./..."},
			ExitCode: 1,
			Stderr:   "build failed\n",
		},
		Platform: Platform{
			OS:           "linux",
			Architecture: "amd64",
		},
		Runtime: &Runtime{
			Kind:    "go",
			Version: SupportedGoVersion,
		},
	}

	if err := manifest.Validate(); err != nil {
		t.Fatalf("legacy v2 manifest rejected: %v", err)
	}

	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}

	var decoded Manifest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.GoDependencies != nil {
		t.Fatal("legacy manifest unexpectedly contains GoDependencies")
	}

	if err := decoded.Validate(); err != nil {
		t.Fatalf("decoded legacy manifest rejected: %v", err)
	}
}
