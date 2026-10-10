package capsule

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestManifestWorkspaceSchemaVersions(t *testing.T) {
	files := []FileMetadata{
		validV2File("src/main.go", 100),
		validV2File("go.mod", 50),
	}

	newManifest := func(version int) Manifest {
		return Manifest{
			SchemaVersion: version,
			Execution: Execution{
				Argv:       []string{"/bin/sh", "-c", "exit 23"},
				WorkingDir: ".",
				ExitCode:   23,
			},
			Platform: Platform{
				OS:           "linux",
				Architecture: "amd64",
			},
			Workspace: Workspace{
				Files: files,
			},
		}
	}

	t.Run("v1 rejects multiple files", func(t *testing.T) {
		if err := newManifest(SchemaVersionV1).Validate(); err == nil {
			t.Fatal("v1 must reject multiple workspace files")
		}
	})

	t.Run("v2 accepts multiple files", func(t *testing.T) {
		if err := newManifest(SchemaVersionV2).Validate(); err != nil {
			t.Fatalf("v2 should accept multiple files: %v", err)
		}
	})

	t.Run("unsupported version rejected", func(t *testing.T) {
		if err := newManifest(3).Validate(); err == nil {
			t.Fatal("unsupported schema version must be rejected")
		}
	})
}

func TestManifestV2RuntimeCompatibility(t *testing.T) {
	newManifest := func() Manifest {
		return Manifest{
			SchemaVersion: SchemaVersionV2,
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
				Files: []FileMetadata{},
			},
		}
	}

	t.Run("legacy capsule omits runtime", func(t *testing.T) {
		m := newManifest()

		data, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}

		var decoded map[string]json.RawMessage
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatal(err)
		}

		if _, exists := decoded["runtime"]; exists {
			t.Fatal("legacy manifest must omit runtime")
		}

		var restored Manifest
		if err := json.Unmarshal(data, &restored); err != nil {
			t.Fatal(err)
		}

		if restored.Runtime != nil {
			t.Fatal("legacy runtime must remain nil")
		}

		if err := restored.Validate(); err != nil {
			t.Fatalf("legacy manifest invalid: %v", err)
		}
	})

	t.Run("go runtime survives serialization", func(t *testing.T) {
		m := newManifest()
		m.Runtime = &Runtime{
			Kind:    "go",
			Version: "1.26.6",
		}

		data, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}

		var restored Manifest
		if err := json.Unmarshal(data, &restored); err != nil {
			t.Fatal(err)
		}

		if restored.Runtime == nil {
			t.Fatal("runtime lost during serialization")
		}

		if restored.Runtime.Kind != "go" ||
			restored.Runtime.Version != "1.26.6" {
			t.Fatalf("unexpected runtime: %+v", restored.Runtime)
		}
	})
}

func TestCapsuleV2RuntimeRoundTrip(t *testing.T) {
	manifest := Manifest{
		SchemaVersion: SchemaVersionV2,
		Execution: Execution{
			Argv:       []string{"go", "build", "./..."},
			WorkingDir: ".",
			ExitCode:   1,
			Stderr:     "build failed\n",
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

	files := []WorkspaceFile{
		{
			Path: "go.mod",
			Data: []byte("module example.com/demo\n"),
			Mode: 0600,
		},
	}

	var archive bytes.Buffer

	if err := WriteWorkspaceV2(&archive, manifest, files); err != nil {
		t.Fatalf("write capsule: %v", err)
	}

	capsulePath := filepath.Join(t.TempDir(), "go-build.bfc")

	if err := os.WriteFile(capsulePath, archive.Bytes(), 0600); err != nil {
		t.Fatalf("save capsule: %v", err)
	}

	verified, err := ReadVerifiedV2(capsulePath)
	if err != nil {
		t.Fatalf("read verified capsule: %v", err)
	}

	if verified.Manifest.Runtime == nil {
		t.Fatal("runtime missing after round trip")
	}

	if got := verified.Manifest.Runtime; got.Kind != "go" ||
		got.Version != SupportedGoVersion {
		t.Fatalf("unexpected runtime: %+v", got)
	}

	if len(verified.Files) != 1 ||
		verified.Files[0].Path != "go.mod" ||
		!bytes.Equal(verified.Files[0].Data, files[0].Data) {
		t.Fatalf("workspace files changed after round trip")
	}
}
