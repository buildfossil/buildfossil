package capsule

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"testing"
)

func TestWriteWorkspaceV2TwoFiles(t *testing.T) {
	m := Manifest{
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
		{Path: "go.mod", Data: []byte("module example.com/test\n"), Mode: 0600},
		{Path: "src/main.go", Data: []byte("package main\n"), Mode: 0600},
	}

	var buf bytes.Buffer

	if err := WriteWorkspaceV2(&buf, m, files); err != nil {
		t.Fatalf("write v2 capsule: %v", err)
	}

	tr := tar.NewReader(bytes.NewReader(buf.Bytes()))

	for _, expected := range []string{
		"manifest.json",
		"workspace/go.mod",
		"workspace/src/main.go",
	} {
		header, err := tr.Next()
		if err != nil {
			t.Fatalf("read %s: %v", expected, err)
		}

		if header.Name != expected {
			t.Fatalf("expected entry %q, got %q", expected, header.Name)
		}

		if _, err := io.Copy(io.Discard, tr); err != nil {
			t.Fatalf("read entry %s: %v", expected, err)
		}
	}

	if _, err := tr.Next(); err != io.EOF {
		t.Fatalf("expected end of archive, got %v", err)
	}
}

func TestWriteWorkspaceV2RejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name    string
		version int
		files   []WorkspaceFile
	}{
		{
			name:    "wrong schema version",
			version: SchemaVersionV1,
			files: []WorkspaceFile{
				{Path: "go.mod", Data: []byte("test"), Mode: 0600},
			},
		},
		{
			name:    "duplicate paths",
			version: SchemaVersionV2,
			files: []WorkspaceFile{
				{Path: "go.mod", Data: []byte("first"), Mode: 0600},
				{Path: "go.mod", Data: []byte("second"), Mode: 0600},
			},
		},
		{
			name:    "parent traversal",
			version: SchemaVersionV2,
			files: []WorkspaceFile{
				{Path: "../secret.txt", Data: []byte("secret"), Mode: 0600},
			},
		},
		{
			name:    "absolute path",
			version: SchemaVersionV2,
			files: []WorkspaceFile{
				{Path: "/etc/passwd", Data: []byte("test"), Mode: 0600},
			},
		},
		{
			name:    "symlink",
			version: SchemaVersionV2,
			files: []WorkspaceFile{
				{Path: "link.txt", Data: []byte("test"), Mode: os.ModeSymlink},
			},
		},
		{
			name:    "total size exceeded",
			version: SchemaVersionV2,
			files: []WorkspaceFile{
				{Path: "a.bin", Data: make([]byte, 10<<20), Mode: 0600},
				{Path: "b.bin", Data: make([]byte, 5<<20), Mode: 0600},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Manifest{
				SchemaVersion: tt.version,
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

			err := WriteWorkspaceV2(&buf, m, tt.files)
			if err == nil {
				t.Fatal("expected invalid input to be rejected")
			}

			if buf.Len() != 0 {
				t.Fatalf(
					"writer produced %d bytes despite validation failure",
					buf.Len(),
				)
			}
		})
	}
}
