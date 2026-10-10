package capsule

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestZeroDependencyV3RoundTrip(t *testing.T) {
	files := []WorkspaceFile{
		{
			Path: "go.mod",
			Data: []byte("module example.com/zero\n\ngo 1.26.6\n"),
			Mode: 0600,
		},
		{
			Path: "go.sum",
			Data: []byte{},
			Mode: 0600,
		},
		{
			Path: "main.go",
			Data: []byte(
				"package main\n\nfunc main() { missingSymbol() }\n",
			),
			Mode: 0600,
		},
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
		GoModules: nil,
	}

	var buffer bytes.Buffer

	if err := WriteWorkspaceV3(
		&buffer,
		manifest,
		files,
		nil,
	); err != nil {
		t.Fatalf("write zero-dependency capsule: %v", err)
	}

	capsulePath := filepath.Join(t.TempDir(), "zero.bfc")
	if err := os.WriteFile(capsulePath, buffer.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}

	verified, err := ReadVerifiedV3(capsulePath)
	if err != nil {
		t.Fatalf("read zero-dependency capsule: %v", err)
	}

	if len(verified.Manifest.GoModules) != 0 {
		t.Fatalf(
			"expected zero Go modules, got %d",
			len(verified.Manifest.GoModules),
		)
	}

	if len(verified.Artifacts) != 0 {
		t.Fatalf(
			"expected zero dependency artifacts, got %d",
			len(verified.Artifacts),
		)
	}

	if len(verified.Files) != len(files) {
		t.Fatalf(
			"expected %d workspace files, got %d",
			len(files),
			len(verified.Files),
		)
	}

	for i := range files {
		if verified.Files[i].Path != files[i].Path {
			t.Fatalf("unexpected workspace path at index %d", i)
		}
		if !bytes.Equal(verified.Files[i].Data, files[i].Data) {
			t.Fatalf("workspace content mismatch: %s", files[i].Path)
		}
	}

	if !strings.Contains(
		verified.Manifest.Execution.Stderr,
		"missingSymbol",
	) {
		t.Fatal("original failure stderr was not preserved")
	}
}
