package capsule

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadVerified(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "valid.bfc")

	manifest := Manifest{
		SchemaVersion: SchemaVersion,
		Execution: Execution{
			Argv:       []string{"/bin/sh", "-c", "exit 17"},
			WorkingDir: ".",
			ExitCode:   17,
		},
		Platform: Platform{
			OS:           "darwin",
			Architecture: "arm64",
		},
	}

	file := WorkspaceFile{
		Path: "fixture.txt",
		Data: []byte("BUILD_FOSSIL_TEST_FAILURE\n"),
		Mode: 0600,
	}

	if err := WriteFile(output, manifest, file); err != nil {
		t.Fatalf("WriteFile(): %v", err)
	}

	t.Run("valid capsule", func(t *testing.T) {
		got, err := ReadVerified(output)
		if err != nil {
			t.Fatalf("ReadVerified(): %v", err)
		}

		if !bytes.Equal(got.File.Data, file.Data) {
			t.Fatal("workspace content mismatch")
		}

		if got.Manifest.Execution.ExitCode != 17 {
			t.Fatal("unexpected exit code")
		}

		if len(got.Manifest.Workspace.Files) != 1 {
			t.Fatal("expected one workspace file")
		}
	})

	t.Run("corrupted workspace", func(t *testing.T) {
		raw, err := os.ReadFile(output)
		if err != nil {
			t.Fatal(err)
		}

		// Replace one byte without changing the tar entry size.
		needle := file.Data
		position := bytes.Index(raw, needle)
		if position < 0 {
			t.Fatal("fixture content not found in archive")
		}

		raw[position] ^= 1

		corrupted := filepath.Join(dir, "corrupted.bfc")
		if err := os.WriteFile(corrupted, raw, 0600); err != nil {
			t.Fatal(err)
		}

		_, err = ReadVerified(corrupted)
		if err == nil {
			t.Fatal("expected corrupted capsule rejection")
		}

		// A tar checksum covers headers, not file contents.
		if !strings.Contains(err.Error(), "SHA-256 mismatch") {
			t.Fatalf("expected SHA-256 mismatch, got: %v", err)
		}
	})
}

func TestReadVerifiedRejectsExtraEntry(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "extra.bfc")

	manifest := Manifest{
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
	}

	file := WorkspaceFile{
		Path: "fixture.txt",
		Data: []byte("BUILD_FOSSIL_TEST_FAILURE\n"),
		Mode: 0600,
	}

	var archive bytes.Buffer
	if err := WriteWithWorkspace(&archive, manifest, file); err != nil {
		t.Fatal(err)
	}

	// Rebuild the archive and append an unexpected third entry.
	var modified bytes.Buffer
	reader := tar.NewReader(bytes.NewReader(archive.Bytes()))
	writer := tar.NewWriter(&modified)

	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}

		data, err := io.ReadAll(reader)
		if err != nil {
			t.Fatal(err)
		}

		if err := writer.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(data); err != nil {
			t.Fatal(err)
		}
	}

	extra := []byte("unexpected")
	if err := writer.WriteHeader(&tar.Header{
		Name:     "workspace/extra.txt",
		Mode:     0600,
		Size:     int64(len(extra)),
		Typeflag: tar.TypeReg,
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := writer.Write(extra); err != nil {
		t.Fatal(err)
	}

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(output, modified.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}

	if _, err := ReadVerified(output); err == nil {
		t.Fatal("expected unexpected archive entry rejection")
	}
}
