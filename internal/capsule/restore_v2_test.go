package capsule

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRestoreWorkspaceV2TwoFiles(t *testing.T) {
	manifest := Manifest{
		SchemaVersion: SchemaVersionV2,
		Execution: Execution{
			Argv:       []string{"/bin/sh", "-c", "exit 23"},
			WorkingDir: ".",
			ExitCode:   23,
		},
		Platform: Platform{
			OS:           "linux",
			Architecture: "amd64",
		},
	}

	files := []WorkspaceFile{
		{
			Path: "go.mod",
			Data: []byte("module example.com/test\n"),
			Mode: 0600,
		},
		{
			Path: "src/main.go",
			Data: []byte("package main\n"),
			Mode: 0600,
		},
	}

	var archive bytes.Buffer
	if err := WriteWorkspaceV2(&archive, manifest, files); err != nil {
		t.Fatalf("write archive: %v", err)
	}

	capsulePath := filepath.Join(t.TempDir(), "test.bfc")
	if err := os.WriteFile(capsulePath, archive.Bytes(), 0600); err != nil {
		t.Fatalf("save archive: %v", err)
	}

	verified, err := ReadVerifiedV2(capsulePath)
	if err != nil {
		t.Fatalf("verify archive: %v", err)
	}

	root := t.TempDir()

	if err := RestoreWorkspaceV2(root, verified); err != nil {
		t.Fatalf("restore workspace: %v", err)
	}

	directory := filepath.Join(root, "src")

	info, err := os.Lstat(directory)
	if err != nil {
		t.Fatalf("stat restored directory: %v", err)
	}

	if !info.IsDir() {
		t.Fatal("expected src to be a directory")
	}

	if info.Mode().Perm() != 0700 {
		t.Fatalf(
			"directory permissions = %o, want 0700",
			info.Mode().Perm(),
		)
	}

	for _, expected := range files {
		destination := filepath.Join(root, filepath.FromSlash(expected.Path))

		actual, err := os.ReadFile(destination)
		if err != nil {
			t.Fatalf("read restored %q: %v", expected.Path, err)
		}

		if !bytes.Equal(actual, expected.Data) {
			t.Fatalf("content mismatch for %q", expected.Path)
		}

		info, err := os.Lstat(destination)
		if err != nil {
			t.Fatalf("stat restored %q: %v", expected.Path, err)
		}

		if !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
			t.Fatalf("unexpected mode for %q: %v", expected.Path, info.Mode())
		}
	}
}

func TestRestoreWorkspaceV2RejectsInvalidInputs(t *testing.T) {
	data := []byte("original content\n")

	makeVerified := func() VerifiedCapsuleV2 {
		return VerifiedCapsuleV2{
			Manifest: Manifest{
				SchemaVersion: SchemaVersionV2,
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
					Files: []FileMetadata{{
						Path:   "src/main.go",
						Size:   int64(len(data)),
						SHA256: SHA256(data),
					}},
				},
			},
			Files: []WorkspaceFile{{
				Path: "src/main.go",
				Data: bytes.Clone(data),
				Mode: 0600,
			}},
		}
	}

	t.Run("corrupted data creates no files", func(t *testing.T) {
		verified := makeVerified()
		verified.Files[0].Data[0] = 'X'

		root := t.TempDir()

		if err := RestoreWorkspaceV2(root, verified); err == nil {
			t.Fatal("expected corrupted data to be rejected")
		}

		entries, err := os.ReadDir(root)
		if err != nil {
			t.Fatal(err)
		}

		if len(entries) != 0 {
			t.Fatal("corrupted data caused filesystem writes")
		}
	})

	t.Run("nonempty destination is rejected", func(t *testing.T) {
		verified := makeVerified()
		root := t.TempDir()

		existing := filepath.Join(root, "existing.txt")
		if err := os.WriteFile(existing, []byte("keep\n"), 0600); err != nil {
			t.Fatal(err)
		}

		if err := RestoreWorkspaceV2(root, verified); err == nil {
			t.Fatal("expected nonempty destination to be rejected")
		}

		content, err := os.ReadFile(existing)
		if err != nil {
			t.Fatal(err)
		}

		if string(content) != "keep\n" {
			t.Fatal("existing file was modified")
		}
	})
}

func TestRestoreWorkspaceV2RejectsPathConflict(t *testing.T) {
	files := []WorkspaceFile{
		{
			Path: "src",
			Data: []byte("file content\n"),
			Mode: 0600,
		},
		{
			Path: "src/main.go",
			Data: []byte("package main\n"),
			Mode: 0600,
		},
	}

	metadata := make([]FileMetadata, 0, len(files))
	for _, file := range files {
		metadata = append(metadata, FileMetadata{
			Path:   file.Path,
			Size:   int64(len(file.Data)),
			SHA256: SHA256(file.Data),
		})
	}

	verified := VerifiedCapsuleV2{
		Manifest: Manifest{
			SchemaVersion: SchemaVersionV2,
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
				Files: metadata,
			},
		},
		Files: files,
	}

	root := t.TempDir()

	err := RestoreWorkspaceV2(root, verified)
	if err == nil {
		t.Fatal("expected file/directory path conflict to be rejected")
	}

	if !strings.Contains(err.Error(), "path conflict") {
		t.Fatalf("expected path conflict error, got: %v", err)
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 0 {
		t.Fatal("path conflict caused filesystem writes")
	}
}

func TestRestoreWorkspaceV2RejectsSymlinkDestination(t *testing.T) {
	realDir := t.TempDir()

	parent := t.TempDir()
	link := filepath.Join(parent, "workspace-link")

	if err := os.Symlink(realDir, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	verified := VerifiedCapsuleV2{
		Manifest: Manifest{
			SchemaVersion: SchemaVersionV2,
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
				Files: []FileMetadata{},
			},
		},
		Files: []WorkspaceFile{},
	}

	if err := RestoreWorkspaceV2(link, verified); err == nil {
		t.Fatal("expected symlink destination to be rejected")
	}

	entries, err := os.ReadDir(realDir)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 0 {
		t.Fatal("symlink destination caused unexpected writes")
	}
}

func TestRestoreWorkspaceV2RejectsIntermediateSymlink(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()

	if err := os.Symlink(outside, filepath.Join(root, "src")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	files := []WorkspaceFile{
		{
			Path: "src/main.go",
			Data: []byte("package main\n"),
			Mode: 0600,
		},
	}

	manifest := Manifest{
		SchemaVersion: SchemaVersionV2,
		Execution: Execution{
			Argv:       []string{"/bin/sh", "-c", "exit 23"},
			WorkingDir: ".",
			ExitCode:   23,
		},
		Platform: Platform{
			OS:           "linux",
			Architecture: "amd64",
		},
	}

	var archive bytes.Buffer

	if err := WriteWorkspaceV2(&archive, manifest, files); err != nil {
		t.Fatalf("write capsule: %v", err)
	}

	capsulePath := filepath.Join(t.TempDir(), "test.bfc")
	if err := os.WriteFile(capsulePath, archive.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}

	verified, err := ReadVerifiedV2(capsulePath)
	if err != nil {
		t.Fatal(err)
	}

	if err := RestoreWorkspaceV2(root, verified); err == nil {
		t.Fatal("expected restore to reject non-empty root")
	}

	if _, err := os.Stat(filepath.Join(outside, "main.go")); !os.IsNotExist(err) {
		t.Fatalf("unexpected file outside workspace: %v", err)
	}
}
