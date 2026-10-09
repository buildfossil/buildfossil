package capsule

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"io"
	"reflect"
	"testing"
)

func TestWriteWithWorkspaceRoundTrip(t *testing.T) {
	manifest := Manifest{
		SchemaVersion: SchemaVersion,
		Execution: Execution{
			Argv:       []string{"/bin/sh", "-c", "exit 17"},
			WorkingDir: ".",
			ExitCode:   17,
			Stderr:     "test failure\n",
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
		t.Fatalf("WriteWithWorkspace() error = %v", err)
	}

	reader := tar.NewReader(bytes.NewReader(archive.Bytes()))

	expected := []string{
		"manifest.json",
		"workspace/fixture.txt",
	}

	for _, name := range expected {
		header, err := reader.Next()
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}

		if header.Name != name {
			t.Fatalf("entry = %q, want %q", header.Name, name)
		}

		if header.Typeflag != tar.TypeReg {
			t.Fatalf("unexpected entry type for %s", name)
		}

		data, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("reading %s content: %v", name, err)
		}

		switch name {
		case "manifest.json":
			var restored Manifest

			if err := json.Unmarshal(data, &restored); err != nil {
				t.Fatalf("decoding manifest: %v", err)
			}

			manifest.Workspace = Workspace{
				Files: []FileMetadata{
					{
						Path:   file.Path,
						Size:   int64(len(file.Data)),
						SHA256: SHA256(file.Data),
					},
				},
			}

			if !reflect.DeepEqual(manifest, restored) {
				t.Error("restored manifest differs from original")
			}

		case "workspace/fixture.txt":
			if !bytes.Equal(data, file.Data) {
				t.Error("restored workspace content differs")
			}

			if header.Mode != int64(file.Mode.Perm()) {
				t.Errorf(
					"file mode = %o, want %o",
					header.Mode,
					file.Mode.Perm(),
				)
			}
		}
	}

	if _, err := reader.Next(); err != io.EOF {
		t.Errorf("expected end of archive, got %v", err)
	}
}
