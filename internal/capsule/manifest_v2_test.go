package capsule

import "testing"

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
