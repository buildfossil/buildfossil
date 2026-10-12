package capsule

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func largeWorkspaceV3Fixture() (Manifest, []WorkspaceFile) {
	files := []WorkspaceFile{
		{
			Path: "go.mod",
			Data: []byte("module example.com/largeworkspace\n\ngo 1.26.6\n"),
			Mode: 0600,
		},
		{
			Path: "go.sum",
			Data: []byte{},
			Mode: 0600,
		},
	}

	for i := 0; i < MaxWorkspaceV3Files-2; i++ {
		files = append(files, WorkspaceFile{
			Path: fmt.Sprintf("src/file-%03d.go", i),
			Data: []byte(fmt.Sprintf(
				"package fixture\n\nvar Value%d = %d\n", i, i,
			)),
			Mode: 0600,
		})
	}

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

	return manifest, files
}

func TestLargeWorkspaceV3RoundTripAndStaging(t *testing.T) {
	manifest, files := largeWorkspaceV3Fixture()

	if len(files) != MaxWorkspaceV3Files {
		t.Fatalf("got %d files, want %d", len(files), MaxWorkspaceV3Files)
	}

	var output bytes.Buffer
	if err := WriteWorkspaceV3(&output, manifest, files, nil); err != nil {
		t.Fatalf("write v3 capsule: %v", err)
	}

	if int64(output.Len()) > MaxCapsuleSize {
		t.Fatalf("capsule exceeds size limit: %d", output.Len())
	}

	capsulePath := filepath.Join(t.TempDir(), "large.bfc")
	if err := os.WriteFile(capsulePath, output.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}

	verified, err := ReadVerifiedV3(capsulePath)
	if err != nil {
		t.Fatalf("read verified v3 capsule: %v", err)
	}

	if len(verified.Files) != MaxWorkspaceV3Files {
		t.Fatalf("verified %d files, want %d",
			len(verified.Files), MaxWorkspaceV3Files)
	}

	for i := range files {
		if verified.Files[i].Path != files[i].Path {
			t.Fatalf("workspace path mismatch at index %d", i)
		}
		if !bytes.Equal(verified.Files[i].Data, files[i].Data) {
			t.Fatalf("workspace data mismatch: %s", files[i].Path)
		}
	}

	snapshot, err := BuildWorkspaceSnapshotV3(verified)
	if err != nil {
		t.Fatalf("build Docker workspace snapshot: %v", err)
	}

	tr := tar.NewReader(bytes.NewReader(snapshot))
	entries := make(map[string][]byte)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read snapshot TAR: %v", err)
		}
		if header.Typeflag != tar.TypeReg {
			t.Fatalf("non-regular snapshot entry: %s", header.Name)
		}
		if _, exists := entries[header.Name]; exists {
			t.Fatalf("duplicate snapshot entry: %s", header.Name)
		}
		content, err := io.ReadAll(tr)
		if err != nil {
			t.Fatal(err)
		}
		entries[header.Name] = content
	}

	if len(entries) != MaxWorkspaceV3Files {
		t.Fatalf("snapshot has %d entries, want %d",
			len(entries), MaxWorkspaceV3Files)
	}

	workspace := t.TempDir()
	for _, file := range verified.Files {
		fullPath := filepath.Join(workspace, filepath.FromSlash(file.Path))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, file.Data, 0600); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(entries[file.Path], file.Data) {
			t.Fatalf("snapshot mismatch for %s", file.Path)
		}
	}

	if err := VerifyStagedWorkspaceV3(workspace, verified.Manifest); err != nil {
		t.Fatalf("verify staged workspace: %v", err)
	}
}

func TestLargeWorkspaceV3RejectsTooManyFiles(t *testing.T) {
	manifest, files := largeWorkspaceV3Fixture()

	files = append(files, WorkspaceFile{
		Path: "src/extra.go",
		Data: []byte("package fixture\n"),
		Mode: 0600,
	})

	var output bytes.Buffer
	if err := WriteWorkspaceV3(&output, manifest, files, nil); err == nil {
		t.Fatal("v3 writer accepted workspace file count above v3 limit")
	}

	if output.Len() != 0 {
		t.Fatal("v3 writer wrote bytes after rejecting too many files")
	}
}
