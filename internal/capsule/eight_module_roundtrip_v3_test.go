package capsule

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func makeEightModuleWorkspaceV3(
	t *testing.T,
) (Manifest, []WorkspaceFile, map[string][]byte) {
	t.Helper()

	manifest := validManifestV3()
	manifest.GoModules = nil

	artifacts := make(map[string][]byte)
	var goMod strings.Builder
	var goSum bytes.Buffer

	goMod.WriteString("module example.com/app\n\ngo 1.26.6\n\nrequire (\n")

	for i := 1; i <= 8; i++ {
		path := fmt.Sprintf("example.com/module%02d", i)

		mod, moduleArtifacts, moduleSum := makeIndexedGoModuleFixture(
			t, i, path,
		)

		manifest.GoModules = append(manifest.GoModules, mod)

		for name, data := range moduleArtifacts {
			artifacts[name] = data
		}

		goSum.Write(moduleSum)
		fmt.Fprintf(&goMod, "\t%s v1.0.0\n", path)
	}

	goMod.WriteString(")\n")

	files := []WorkspaceFile{
		{
			Path: "go.mod",
			Data: []byte(goMod.String()),
			Mode: 0644,
		},
		{
			Path: "go.sum",
			Data: goSum.Bytes(),
			Mode: 0644,
		},
		{
			Path: "main.go",
			Data: []byte(
				"package main\nfunc main() { undefinedSymbol() }\n",
			),
			Mode: 0644,
		},
	}

	return manifest, files, artifacts
}

func TestWriteReadV3EightModulesRoundTrip(t *testing.T) {
	manifest, files, artifacts := makeEightModuleWorkspaceV3(t)

	var output bytes.Buffer

	if err := WriteWorkspaceV3(
		&output, manifest, files, artifacts,
	); err != nil {
		t.Fatalf("eight-module writer rejected valid capsule: %v", err)
	}

	filename := filepath.Join(t.TempDir(), "eight.bfc")
	if err := os.WriteFile(filename, output.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}

	verified, err := ReadVerifiedV3(filename)
	if err != nil {
		t.Fatalf("eight-module reader rejected capsule: %v", err)
	}

	if got := len(verified.Manifest.GoModules); got != 8 {
		t.Fatalf("module count = %d; want 8", got)
	}

	if got := len(verified.Artifacts); got != 24 {
		t.Fatalf("artifact count = %d; want 24", got)
	}

	if err := VerifyGoModuleArtifactSetV3(
		verified.Manifest.GoModules,
		verified.Artifacts,
		files[1].Data,
	); err != nil {
		t.Fatalf("eight-module verification failed: %v", err)
	}
}

func TestWriteV3NineModulesRejectsWithoutOutput(t *testing.T) {
	manifest, files, artifacts := makeEightModuleWorkspaceV3(t)

	ninth, ninthArtifacts, ninthSum := makeIndexedGoModuleFixture(
		t, 9, "example.com/module09",
	)

	manifest.GoModules = append(manifest.GoModules, ninth)

	for name, data := range ninthArtifacts {
		artifacts[name] = data
	}

	files[0].Data = bytes.Replace(
		files[0].Data,
		[]byte(")\n"),
		[]byte("\texample.com/module09 v1.0.0\n)\n"),
		1,
	)
	files[1].Data = append(files[1].Data, ninthSum...)

	var output bytes.Buffer

	err := WriteWorkspaceV3(&output, manifest, files, artifacts)

	if err == nil {
		t.Fatal("writer accepted nine modules")
	}
	if output.Len() != 0 {
		t.Fatalf("writer emitted %d bytes before rejection", output.Len())
	}
}
