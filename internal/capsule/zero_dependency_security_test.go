package capsule

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func zeroDependencyFixtureV3(t *testing.T) []byte {
	t.Helper()

	manifest := Manifest{
		SchemaVersion: SchemaVersionV3,
		Runtime: &Runtime{
			Kind:    "go",
			Version: SupportedGoVersion,
		},
		GoBuildEnv: &GoBuildEnvironment{
			GOOS:       "linux",
			GOARCH:     "amd64",
			CGOEnabled: "0",
		},
		Platform: Platform{
			OS:           "darwin",
			Architecture: "arm64",
		},
		Execution: Execution{
			Argv:       []string{"go", "build", "./..."},
			WorkingDir: ".",
			ExitCode:   1,
			Stderr:     "undefined: missingSymbol\n",
		},
	}

	files := []WorkspaceFile{
		{
			Path: "go.mod",
			Data: []byte("module example.com/zero\n\ngo 1.26.6\n"),
			Mode: 0600,
		},
		{
			Path: "go.sum",
			Data: nil,
			Mode: 0600,
		},
		{
			Path: "main.go",
			Data: []byte("package main\nfunc main() { missingSymbol() }\n"),
			Mode: 0600,
		},
	}

	var out bytes.Buffer

	if err := WriteWorkspaceV3(&out, manifest, files, nil); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	return out.Bytes()
}

func TestZeroDependencyV3RejectsExtraDependencyEntry(t *testing.T) {
	original := zeroDependencyFixtureV3(t)

	// Rebuild TAR with an additional undeclared dependency entry.
	reader := tar.NewReader(bytes.NewReader(original))
	var modified bytes.Buffer
	writer := tar.NewWriter(&modified)

	for {
		header, err := reader.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			t.Fatalf("read fixture TAR: %v", err)
		}

		data := make([]byte, header.Size)
		if _, err := io.ReadFull(reader, data); err != nil {
			t.Fatalf("read TAR entry: %v", err)
		}

		copyHeader := *header

		if err := writer.WriteHeader(&copyHeader); err != nil {
			t.Fatal(err)
		}

		if _, err := writer.Write(data); err != nil {
			t.Fatal(err)
		}
	}

	payload := []byte("unexpected artifact")

	if err := writer.WriteHeader(&tar.Header{
		Name:     "dependencies/module-0001.zip",
		Mode:     0600,
		Size:     int64(len(payload)),
		Typeflag: tar.TypeReg,
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := writer.Write(payload); err != nil {
		t.Fatal(err)
	}

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "malicious.bfc")
	if err := os.WriteFile(path, modified.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}

	if _, err := ReadVerifiedV3(path); err == nil {
		t.Fatal("reader accepted undeclared dependency entry")
	}
}
