package capsule

import (
	"archive/tar"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestReadManifestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "failure.bfc")

	original := Manifest{
		SchemaVersion: SchemaVersion,
		Execution: Execution{
			Argv:       []string{"/bin/sh", "-c", "exit 17"},
			WorkingDir: ".",
			ExitCode:   17,
			Stderr:     "BUILD_FOSSIL_TEST_FAILURE\n",
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

	if err := WriteFile(output, original, file); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	original.Workspace = Workspace{
		Files: []FileMetadata{
			{
				Path:   file.Path,
				Size:   int64(len(file.Data)),
				SHA256: SHA256(file.Data),
			},
		},
	}

	restored, err := ReadManifest(output)
	if err != nil {
		t.Fatalf("ReadManifest() error = %v", err)
	}

	if !reflect.DeepEqual(original, restored) {
		t.Errorf("restored manifest differs from original")
	}
}

func TestReadManifestRejectsMissingManifest(t *testing.T) {
	output := filepath.Join(t.TempDir(), "invalid.bfc")

	f, err := os.Create(output)
	if err != nil {
		t.Fatal(err)
	}

	tw := tar.NewWriter(f)

	data := []byte("not a manifest")

	err = tw.WriteHeader(&tar.Header{
		Name:     "wrong.txt",
		Mode:     0600,
		Size:     int64(len(data)),
		Typeflag: tar.TypeReg,
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := tw.Write(data); err != nil {
		t.Fatal(err)
	}

	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	_, err = ReadManifest(output)
	if err == nil {
		t.Fatal("expected missing manifest rejection")
	}

	if !strings.Contains(err.Error(), "invalid manifest entry") {
		t.Errorf("unexpected error: %v", err)
	}
}
