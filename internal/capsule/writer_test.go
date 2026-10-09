package capsule

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"io"
	"reflect"
	"testing"
)

func TestWriteManifestRoundTrip(t *testing.T) {
	original := Manifest{
		SchemaVersion: SchemaVersion,
		Execution: Execution{
			Argv:       []string{"go", "test", "./..."},
			WorkingDir: ".",
			ExitCode:   1,
			Stdout:     "running tests\n",
			Stderr:     "test failed\n",
		},
		Platform: Platform{
			OS:           "linux",
			Architecture: "amd64",
		},
	}

	var archive bytes.Buffer

	if err := Write(&archive, original); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	reader := tar.NewReader(bytes.NewReader(archive.Bytes()))

	header, err := reader.Next()
	if err != nil {
		t.Fatalf("reading tar header: %v", err)
	}

	if header.Name != "manifest.json" {
		t.Fatalf("entry = %q, want manifest.json", header.Name)
	}

	if header.Typeflag != tar.TypeReg {
		t.Fatalf("unexpected tar entry type: %d", header.Typeflag)
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("reading manifest: %v", err)
	}

	var restored Manifest
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("decoding manifest: %v", err)
	}

	if !reflect.DeepEqual(original, restored) {
		t.Errorf("restored manifest differs from original")
	}

	_, err = reader.Next()
	if err != io.EOF {
		t.Errorf("expected end of archive, got %v", err)
	}
}
