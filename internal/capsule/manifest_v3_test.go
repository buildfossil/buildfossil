package capsule

import (
	"encoding/json"
	"strings"
	"testing"
)

func validManifestV3() Manifest {
	return Manifest{
		SchemaVersion: SchemaVersionV3,
		Execution: Execution{
			Argv:       []string{"go", "build", "./..."},
			WorkingDir: ".",
			ExitCode:   1,
		},
		Platform: Platform{
			OS:           "linux",
			Architecture: "amd64",
		},
		Workspace: Workspace{
			Files: nil,
		},
		Runtime: &Runtime{
			Kind:    "go",
			Version: SupportedGoVersion,
		},
		GoBuildEnv: &GoBuildEnvironment{
			GOOS:       "linux",
			GOARCH:     "amd64",
			CGOEnabled: "0",
		},
		GoModules: []GoModuleV3{
			validGoModuleV3(),
		},
	}
}

func TestManifestV3Valid(t *testing.T) {
	m := validManifestV3()

	if err := m.Validate(); err != nil {
		t.Fatalf("valid v3 manifest rejected: %v", err)
	}
}

func TestManifestV3JSONRoundTrip(t *testing.T) {
	original := validManifestV3()

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}

	var restored Manifest
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}

	if restored.SchemaVersion != SchemaVersionV3 {
		t.Fatalf(
			"schema = %d; want %d",
			restored.SchemaVersion,
			SchemaVersionV3,
		)
	}

	if len(restored.GoModules) != 1 {
		t.Fatalf(
			"modules = %d; want 1",
			len(restored.GoModules),
		)
	}

	if err := restored.Validate(); err != nil {
		t.Fatalf("restored manifest rejected: %v", err)
	}
}

func TestManifestV3RejectsInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Manifest)
	}{
		{
			name: "modules in v1",
			mutate: func(m *Manifest) {
				m.SchemaVersion = SchemaVersionV1
			},
		},
		{
			name: "modules in v2",
			mutate: func(m *Manifest) {
				m.SchemaVersion = SchemaVersionV2
			},
		},
		{
			name: "missing runtime",
			mutate: func(m *Manifest) {
				m.Runtime = nil
			},
		},
		{
			name: "unsupported runtime",
			mutate: func(m *Manifest) {
				m.Runtime.Version = "1.25.0"
			},
		},
		{
			name: "missing Go environment",
			mutate: func(m *Manifest) {
				m.GoBuildEnv = nil
			},
		},
		{
			name: "invalid module path",
			mutate: func(m *Manifest) {
				m.GoModules[0].Path = "../evil"
			},
		},
		{
			name: "invalid artifact hash",
			mutate: func(m *Manifest) {
				m.GoModules[0].Artifacts[0].SHA256 =
					"sha256:" + strings.Repeat("z", 64)
			},
		},
		{
			name: "unsupported schema",
			mutate: func(m *Manifest) {
				m.SchemaVersion = 99
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := validManifestV3()
			tt.mutate(&m)

			if err := m.Validate(); err == nil {
				t.Fatal("invalid manifest accepted")
			}
		})
	}
}

func TestManifestV3UsesWorkspaceV2Limits(t *testing.T) {
	m := validManifestV3()

	for i := 0; i <= MaxWorkspaceV2Files; i++ {
		m.Workspace.Files = append(m.Workspace.Files, FileMetadata{
			Path:   "file-" + strings.Repeat("x", i),
			Size:   1,
			SHA256: "sha256:" + strings.Repeat("a", 64),
		})
	}

	if err := m.Validate(); err == nil {
		t.Fatal("v3 accepted workspace exceeding v2 file count limit")
	}
}

func TestManifestV3AllowsEmptyModules(t *testing.T) {
	m := validManifestV3()
	m.GoModules = nil

	if err := m.Validate(); err != nil {
		t.Fatalf("v3 with empty modules rejected: %v", err)
	}
}
