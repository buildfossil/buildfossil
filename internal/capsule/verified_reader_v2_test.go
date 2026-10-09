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

func TestReadVerifiedV2TwoFiles(t *testing.T) {
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

	var buf bytes.Buffer

	if err := WriteWorkspaceV2(&buf, manifest, files); err != nil {
		t.Fatalf("write v2 archive: %v", err)
	}

	path := filepath.Join(t.TempDir(), "test.bfc")
	if err := os.WriteFile(path, buf.Bytes(), 0600); err != nil {
		t.Fatalf("save archive: %v", err)
	}

	verified, err := ReadVerifiedV2(path)
	if err != nil {
		t.Fatalf("read verified v2 archive: %v", err)
	}

	if verified.Manifest.SchemaVersion != SchemaVersionV2 {
		t.Fatalf("unexpected schema version")
	}

	if len(verified.Files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(verified.Files))
	}

	for i, expected := range files {
		actual := verified.Files[i]

		if actual.Path != expected.Path {
			t.Fatalf("file %d: path mismatch", i)
		}

		if !bytes.Equal(actual.Data, expected.Data) {
			t.Fatalf("file %d: content mismatch", i)
		}
	}
}

func TestReadVerifiedV2RejectsCorruptedFile(t *testing.T) {
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

	original := []byte("UNIQUE_ORIGINAL_CONTENT_12345")

	var buf bytes.Buffer

	err := WriteWorkspaceV2(&buf, manifest, []WorkspaceFile{
		{
			Path: "src/main.go",
			Data: original,
			Mode: 0600,
		},
	})
	if err != nil {
		t.Fatalf("write capsule: %v", err)
	}

	archive := bytes.Clone(buf.Bytes())

	// Change file contents without updating its digest.
	oldData := []byte("UNIQUE_ORIGINAL_CONTENT_12345")
	newData := []byte("UNIQUE_MODIFIED_CONTENT_12345")

	if len(oldData) != len(newData) {
		t.Fatal("test data lengths must match")
	}

	if bytes.Count(archive, oldData) != 1 {
		t.Fatal("expected exactly one occurrence of original content")
	}

	archive = bytes.Replace(archive, oldData, newData, 1)

	path := filepath.Join(t.TempDir(), "corrupted.bfc")

	if err := os.WriteFile(path, archive, 0600); err != nil {
		t.Fatalf("save corrupted capsule: %v", err)
	}

	_, err = ReadVerifiedV2(path)
	if err == nil {
		t.Fatal("expected corrupted workspace file to be rejected")
	}

	if !strings.Contains(err.Error(), "SHA-256 mismatch") {
		t.Fatalf("expected SHA-256 mismatch, got: %v", err)
	}
}

func TestReadVerifiedV2RejectsExtraTarEntry(t *testing.T) {
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

	var buf bytes.Buffer

	err := WriteWorkspaceV2(&buf, manifest, []WorkspaceFile{
		{
			Path: "go.mod",
			Data: []byte("module example.com/test\n"),
			Mode: 0600,
		},
	})
	if err != nil {
		t.Fatalf("write capsule: %v", err)
	}

	// Append a second tar archive containing an unexpected file.
	var extra bytes.Buffer
	tw := tar.NewWriter(&extra)

	content := []byte("unexpected content")

	err = tw.WriteHeader(&tar.Header{
		Name:     "workspace/secrets.txt",
		Mode:     0600,
		Size:     int64(len(content)),
		Typeflag: tar.TypeReg,
	})
	if err != nil {
		t.Fatalf("write extra header: %v", err)
	}

	if _, err := tw.Write(content); err != nil {
		t.Fatalf("write extra content: %v", err)
	}

	if err := tw.Close(); err != nil {
		t.Fatalf("close extra archive: %v", err)
	}

	archive := append(bytes.Clone(buf.Bytes()), extra.Bytes()...)

	path := filepath.Join(t.TempDir(), "extra.bfc")
	if err := os.WriteFile(path, archive, 0600); err != nil {
		t.Fatalf("save archive: %v", err)
	}

	if _, err := ReadVerifiedV2(path); err == nil {
		t.Fatal("expected archive with extra data to be rejected")
	}
}

func TestReadVerifiedV2RejectsCorruptedTarTrailer(t *testing.T) {
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

	var buf bytes.Buffer

	err := WriteWorkspaceV2(&buf, manifest, []WorkspaceFile{
		{
			Path: "go.mod",
			Data: []byte("module example.com/test\n"),
			Mode: 0600,
		},
	})
	if err != nil {
		t.Fatalf("write capsule: %v", err)
	}

	archive := bytes.Clone(buf.Bytes())

	// Corrupt the final tar block without changing archive size.
	archive[len(archive)-1] = 0xFF

	path := filepath.Join(t.TempDir(), "corrupted-trailer.bfc")
	if err := os.WriteFile(path, archive, 0600); err != nil {
		t.Fatalf("save archive: %v", err)
	}

	_, err = ReadVerifiedV2(path)
	if err == nil {
		t.Fatal("expected corrupted tar trailer to be rejected")
	}

	t.Logf("Reader rejected corrupted trailer: %v", err)
}

func TestReadVerifiedV2RejectsMismatchedTarPath(t *testing.T) {
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

	var buf bytes.Buffer

	err := WriteWorkspaceV2(&buf, manifest, []WorkspaceFile{
		{
			Path: "go.mod",
			Data: []byte("module example.com/test\n"),
			Mode: 0600,
		},
	})
	if err != nil {
		t.Fatalf("write capsule: %v", err)
	}

	archive := bytes.Clone(buf.Bytes())

	// Replace the tar entry name with an equally long name.
	oldName := []byte("workspace/go.mod")
	newName := []byte("workspace/evilmd")

	if len(oldName) != len(newName) {
		t.Fatal("test path lengths must match")
	}

	if bytes.Count(archive, oldName) != 1 {
		t.Fatal("expected exactly one tar path occurrence")
	}

	archive = bytes.Replace(archive, oldName, newName, 1)

	path := filepath.Join(t.TempDir(), "mismatched-path.bfc")
	if err := os.WriteFile(path, archive, 0600); err != nil {
		t.Fatalf("save archive: %v", err)
	}

	_, err = ReadVerifiedV2(path)
	if err == nil {
		t.Fatal("expected mismatched tar path to be rejected")
	}

	t.Logf("Reader rejected mismatched path: %v", err)
}

func TestReadVerifiedV2RejectsValidTarWithWrongPath(t *testing.T) {
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

	var original bytes.Buffer

	if err := WriteWorkspaceV2(&original, manifest, []WorkspaceFile{
		{
			Path: "go.mod",
			Data: []byte("module example.com/test\n"),
			Mode: 0600,
		},
	}); err != nil {
		t.Fatal(err)
	}

	// Rebuild a structurally valid tar archive, changing only
	// the workspace entry name. Keep manifest.json unchanged.
	tr := tar.NewReader(bytes.NewReader(original.Bytes()))

	var modified bytes.Buffer
	tw := tar.NewWriter(&modified)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read original tar: %v", err)
		}

		content, err := io.ReadAll(tr)
		if err != nil {
			t.Fatal(err)
		}

		copied := *header
		if copied.Name == "workspace/go.mod" {
			copied.Name = "workspace/evilmd"
		}

		if err := tw.WriteHeader(&copied); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(content); err != nil {
			t.Fatal(err)
		}
	}

	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "wrong-path.bfc")

	if err := os.WriteFile(path, modified.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := ReadVerifiedV2(path)
	if err == nil {
		t.Fatal("expected mismatched path to be rejected")
	}

	if !strings.Contains(err.Error(), "invalid workspace entry") {
		t.Fatalf("expected path mismatch rejection, got: %v", err)
	}

	t.Logf("Correctly rejected: %v", err)
}

func TestReadVerifiedV1RejectsV2(t *testing.T) {
	data := []byte("version boundary\n")

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
			Path: "fixture.txt",
			Data: data,
			Mode: 0600,
		},
	}

	var archive bytes.Buffer

	if err := WriteWorkspaceV2(&archive, manifest, files); err != nil {
		t.Fatalf("write v2 archive: %v", err)
	}

	filename := filepath.Join(t.TempDir(), "v2.bfc")

	if err := os.WriteFile(filename, archive.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}

	if _, err := ReadVerifiedV2(filename); err != nil {
		t.Fatalf("v2 reader rejected valid v2 archive: %v", err)
	}

	_, err := ReadVerified(filename)
	if err == nil {
		t.Fatal("v1 reader accepted v2 archive")
	}

	if !strings.Contains(err.Error(), "expected schema version 1") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReadVerifiedV2RejectsV1(t *testing.T) {
	data := []byte("version boundary\n")

	manifest := Manifest{
		SchemaVersion: SchemaVersionV1,
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

	file := WorkspaceFile{
		Path: "fixture.txt",
		Data: data,
		Mode: 0600,
	}

	filename := filepath.Join(t.TempDir(), "v1.bfc")

	if err := WriteFile(filename, manifest, file); err != nil {
		t.Fatalf("write v1 archive: %v", err)
	}

	if _, err := ReadVerified(filename); err != nil {
		t.Fatalf("v1 reader rejected valid v1 archive: %v", err)
	}

	_, err := ReadVerifiedV2(filename)
	if err == nil {
		t.Fatal("v2 reader accepted v1 archive")
	}

	if !strings.Contains(err.Error(), "expected schema version 2") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWriteWorkspaceV2RejectsLongPath(t *testing.T) {
	longName := strings.Repeat("a", 110) + ".txt"
	data := []byte("long path test\n")

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
			Path: longName,
			Data: data,
			Mode: 0600,
		},
	}

	var archive bytes.Buffer

	err := WriteWorkspaceV2(&archive, manifest, files)
	if err == nil {
		t.Fatal("expected long workspace path to be rejected")
	}

	if !strings.Contains(err.Error(), "canonical TAR limit") {
		t.Fatalf("unexpected error: %v", err)
	}

	if archive.Len() != 0 {
		t.Fatal("writer produced output for invalid workspace path")
	}
}

func TestReadVerifiedV2MaxPathRoundTrip(t *testing.T) {
	name := strings.Repeat("a", MaxWorkspaceV2PathBytes)
	data := []byte("boundary test\n")

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
			Path: name,
			Data: data,
			Mode: 0600,
		},
	}

	var archive bytes.Buffer

	if err := WriteWorkspaceV2(&archive, manifest, files); err != nil {
		t.Fatalf("write archive: %v", err)
	}

	filename := filepath.Join(t.TempDir(), "boundary.bfc")

	if err := os.WriteFile(filename, archive.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}

	verified, err := ReadVerifiedV2(filename)
	if err != nil {
		t.Fatalf("read archive: %v", err)
	}

	if len(verified.Files) != 1 {
		t.Fatalf("file count = %d, want 1", len(verified.Files))
	}

	if verified.Files[0].Path != name {
		t.Fatalf("path mismatch: %q", verified.Files[0].Path)
	}

	if !bytes.Equal(verified.Files[0].Data, data) {
		t.Fatal("content mismatch")
	}
}
