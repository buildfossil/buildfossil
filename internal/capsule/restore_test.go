package capsule

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRestoreWorkspace(t *testing.T) {
	data := []byte("BUILD_FOSSIL_TEST_FAILURE\n")

	verified := VerifiedCapsule{
		Manifest: Manifest{
			SchemaVersion: SchemaVersion,
			Execution: Execution{
				Argv:       []string{"/bin/sh", "-c", "exit 17"},
				WorkingDir: ".",
				ExitCode:   17,
			},
			Platform: Platform{
				OS:           "linux",
				Architecture: "amd64",
			},
			Workspace: Workspace{
				Files: []FileMetadata{{
					Path:   "fixture.txt",
					Size:   int64(len(data)),
					SHA256: SHA256(data),
				}},
			},
		},
		File: WorkspaceFile{
			Path: "fixture.txt",
			Data: data,
			Mode: 0600,
		},
	}

	t.Run("successful restore", func(t *testing.T) {
		root := t.TempDir()

		if err := RestoreWorkspace(root, verified); err != nil {
			t.Fatalf("RestoreWorkspace(): %v", err)
		}

		path := filepath.Join(root, "fixture.txt")
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}

		if string(got) != string(data) {
			t.Errorf("restored data = %q, want %q", got, data)
		}

		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}

		if info.Mode().Perm() != 0600 {
			t.Errorf("permissions = %o, want 0600", info.Mode().Perm())
		}
	})

	t.Run("nonempty destination", func(t *testing.T) {
		root := t.TempDir()

		if err := os.WriteFile(
			filepath.Join(root, "existing.txt"),
			[]byte("keep"),
			0600,
		); err != nil {
			t.Fatal(err)
		}

		if err := RestoreWorkspace(root, verified); err == nil {
			t.Fatal("expected nonempty directory rejection")
		}
	})

	t.Run("modified data", func(t *testing.T) {
		root := t.TempDir()
		corrupted := verified
		corrupted.File.Data = []byte("BUILD_FOSSIL_TEST_SUCCESS\n")

		if err := RestoreWorkspace(root, corrupted); err == nil {
			t.Fatal("expected SHA-256 mismatch rejection")
		}

		entries, err := os.ReadDir(root)
		if err != nil {
			t.Fatal(err)
		}

		if len(entries) != 0 {
			t.Fatal("corrupted data created files")
		}
	})
}
