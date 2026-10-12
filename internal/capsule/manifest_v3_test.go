package capsule

import (
	"encoding/json"
	"fmt"
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

func TestManifestV3WorkspaceFileLimits(t *testing.T) {
	makeFiles := func(count int) []FileMetadata {
		files := make([]FileMetadata, 0, count)
		for i := 0; i < count; i++ {
			files = append(files, FileMetadata{
				Path:   fmt.Sprintf("file-%03d", i),
				Size:   1,
				SHA256: "sha256:" + strings.Repeat("a", 64),
			})
		}
		return files
	}

	for _, tc := range []struct {
		name    string
		count   int
		wantErr bool
	}{
		{"64 files", 64, false},
		{"65 files", 65, false},
		{"256 files", 256, false},
		{"257 files", 257, false},
		{"2048 files", 2048, false},
		{"2049 files", 2049, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := validManifestV3()
			m.Workspace.Files = makeFiles(tc.count)

			err := m.Validate()
			if (err != nil) != tc.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestManifestV3AllowsEmptyModules(t *testing.T) {
	m := validManifestV3()
	m.GoModules = nil

	if err := m.Validate(); err != nil {
		t.Fatalf("v3 with empty modules rejected: %v", err)
	}
}
