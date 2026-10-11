package capsule

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestMultiModuleV3WriterReaderRoundTrip(t *testing.T) {
	first, firstArtifacts, firstSum := makeIndexedGoModuleFixture(
		t, 1, "example.com/a",
	)
	second, secondArtifacts, secondSum := makeIndexedGoModuleFixture(
		t, 2, "example.com/b",
	)

	artifacts := make(map[string][]byte)
	for name, data := range firstArtifacts {
		artifacts[name] = data
	}
	for name, data := range secondArtifacts {
		artifacts[name] = data
	}

	goMod := []byte(`module example.com/app

go 1.26.6

require (
	example.com/a v1.0.0
	example.com/b v1.0.0
)
`)
	goSum := append(append([]byte(nil), firstSum...), secondSum...)

	files := []WorkspaceFile{
		{Path: "go.mod", Data: goMod, Mode: 0644},
		{Path: "go.sum", Data: goSum, Mode: 0644},
		{
			Path: "main.go",
			Data: []byte("package main\nfunc main() { undefinedSymbol() }\n"),
			Mode: 0644,
		},
	}

	manifest := Manifest{
		SchemaVersion: SchemaVersionV3,
		Runtime: &Runtime{
			Kind: "go", Version: SupportedGoVersion,
		},
		GoBuildEnv: &GoBuildEnvironment{
			GOOS:       "linux",
			GOARCH:     "amd64",
			CGOEnabled: "0",
		},
		GoModules: []GoModuleV3{first, second},
		Execution: Execution{
			Argv:       []string{"go", "build", "./..."},
			WorkingDir: ".",
			ExitCode:   1,
			Stderr:     "undefinedSymbol\n",
		},
		Platform: Platform{
			OS: "darwin", Architecture: "arm64",
		},
	}

	var archive bytes.Buffer
	if err := WriteWorkspaceV3(
		&archive, manifest, files, artifacts,
	); err != nil {
		t.Fatalf("write two-module capsule: %v", err)
	}

	filename := filepath.Join(t.TempDir(), "multi.bfc")
	if err := os.WriteFile(filename, archive.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}

	verified, err := ReadVerifiedV3(filename)
	if err != nil {
		t.Fatalf("read two-module capsule: %v", err)
	}

	if len(verified.Manifest.GoModules) != 2 {
		t.Fatalf("expected two modules, got %d",
			len(verified.Manifest.GoModules))
	}

	if len(verified.Artifacts) != 6 {
		t.Fatalf("expected six artifacts, got %d",
			len(verified.Artifacts))
	}

	if err := VerifyGoModuleArtifactSetV3(
		verified.Manifest.GoModules,
		verified.Artifacts,
		goSum,
	); err != nil {
		t.Fatalf("roundtrip verification failed: %v", err)
	}
}
