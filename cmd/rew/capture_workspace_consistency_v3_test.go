package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rewtools/rew/internal/capsule"
)

func TestCompareCapturedWorkspaceV3(t *testing.T) {
	before := []capsule.WorkspaceFile{
		{
			Path: "main.go",
			Data: []byte("package main\nfunc main() {}\n"),
			Mode: 0600,
		},
	}

	unchanged := []capsule.WorkspaceFile{
		{
			Path: "main.go",
			Data: append([]byte(nil), before[0].Data...),
			Mode: 0600,
		},
	}

	if err := compareCapturedWorkspaceV3(before, unchanged); err != nil {
		t.Fatalf("unchanged workspace rejected: %v", err)
	}

	changed := []capsule.WorkspaceFile{
		{
			Path: "main.go",
			Data: []byte("package main\nfunc main() { panic(1) }\n"),
			Mode: 0600,
		},
	}

	err := compareCapturedWorkspaceV3(before, changed)
	if err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("changed workspace accepted: %v", err)
	}
}

func TestVerifyCapturedWorkspaceV3RejectsChangedFile(t *testing.T) {
	workspace := t.TempDir()
	path := filepath.Join(workspace, "main.go")

	original := []byte("package main\nfunc main() { missingSymbol() }\n")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}

	before, err := capsule.ReadWorkspaceFilesV2(
		workspace,
		[]string{"main.go"},
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(
		path,
		[]byte("package main\nfunc main() { differentSymbol() }\n"),
		0600,
	); err != nil {
		t.Fatal(err)
	}

	if err := verifyCapturedWorkspaceV3(
		workspace,
		[]string{"main.go"},
		before,
	); err == nil {
		t.Fatal("workspace mutation was accepted")
	}
}
