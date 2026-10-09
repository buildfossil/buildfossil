package capsule

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestReadWorkspaceFilesV2TwoFiles(t *testing.T) {
	root := t.TempDir()

	if err := os.Mkdir(
		filepath.Join(root, "src"),
		0700,
	); err != nil {
		t.Fatal(err)
	}

	inputs := []struct {
		path string
		data []byte
	}{
		{
			path: "go.mod",
			data: []byte("module example.com/app\n"),
		},
		{
			path: "src/main.go",
			data: []byte("package main\n"),
		},
	}

	for _, input := range inputs {
		destination := filepath.Join(
			root,
			filepath.FromSlash(input.path),
		)

		if err := os.WriteFile(destination, input.data, 0600); err != nil {
			t.Fatal(err)
		}
	}

	files, err := ReadWorkspaceFilesV2(
		root,
		[]string{"go.mod", "src/main.go"},
	)
	if err != nil {
		t.Fatalf("ReadWorkspaceFilesV2(): %v", err)
	}

	if len(files) != len(inputs) {
		t.Fatalf(
			"file count = %d, want %d",
			len(files),
			len(inputs),
		)
	}

	for i, file := range files {
		if file.Path != inputs[i].path {
			t.Errorf(
				"file %d path = %q, want %q",
				i,
				file.Path,
				inputs[i].path,
			)
		}

		if !bytes.Equal(file.Data, inputs[i].data) {
			t.Errorf("file %d content mismatch", i)
		}

		if file.Mode.Perm() != 0600 {
			t.Errorf(
				"file %d mode = %o, want 0600",
				i,
				file.Mode.Perm(),
			)
		}
	}
}

func TestReadWorkspaceFilesV2RejectsSymlinks(t *testing.T) {
	t.Run("symlink file", func(t *testing.T) {
		root := t.TempDir()
		outside := t.TempDir()

		target := filepath.Join(outside, "secret.txt")
		if err := os.WriteFile(target, []byte("secret\n"), 0600); err != nil {
			t.Fatal(err)
		}

		link := filepath.Join(root, "config.txt")
		if err := os.Symlink(target, link); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}

		_, err := ReadWorkspaceFilesV2(root, []string{"config.txt"})
		if err == nil {
			t.Fatal("expected symlink file to be rejected")
		}
	})

	t.Run("symlink parent directory", func(t *testing.T) {
		root := t.TempDir()
		outside := t.TempDir()

		target := filepath.Join(outside, "main.go")
		if err := os.WriteFile(target, []byte("package main\n"), 0600); err != nil {
			t.Fatal(err)
		}

		link := filepath.Join(root, "src")
		if err := os.Symlink(outside, link); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}

		_, err := ReadWorkspaceFilesV2(root, []string{"src/main.go"})
		if err == nil {
			t.Fatal("expected symlink parent directory to be rejected")
		}
	})
}

func TestReadWorkspaceFilesV2RejectsTraversal(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()

	secret := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secret, []byte("PRIVATE\n"), 0600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		path string
	}{
		{
			name: "parent traversal",
			path: "../secret.txt",
		},
		{
			name: "nested traversal",
			path: "src/../../secret.txt",
		},
		{
			name: "absolute path",
			path: secret,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, err := ReadWorkspaceFilesV2(
				root,
				[]string{tt.path},
			)

			if err == nil {
				t.Fatalf("expected path %q to be rejected", tt.path)
			}

			if len(files) != 0 {
				t.Fatalf("unexpected files returned: %d", len(files))
			}
		})
	}

	content, err := os.ReadFile(secret)
	if err != nil {
		t.Fatal(err)
	}

	if string(content) != "PRIVATE\n" {
		t.Fatal("external file was modified")
	}
}
