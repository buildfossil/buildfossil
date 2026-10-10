package capsule

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/mod/module"
)

func makeCaptureModuleCacheV3(t *testing.T) (
	[]byte,
	[]byte,
	string,
) {
	t.Helper()

	mod, artifacts, goSum := makeGoModuleFixture(t)

	escapedPath, err := module.EscapePath(mod.Path)
	if err != nil {
		t.Fatal(err)
	}

	escapedVersion, err := module.EscapeVersion(mod.Version)
	if err != nil {
		t.Fatal(err)
	}

	cacheRoot := t.TempDir()

	versionDir := filepath.Join(
		cacheRoot,
		"cache",
		"download",
		filepath.FromSlash(escapedPath),
		"@v",
	)

	if err := os.MkdirAll(versionDir, 0700); err != nil {
		t.Fatal(err)
	}

	for _, ext := range []string{".zip", ".mod", ".info"} {
		if err := os.WriteFile(
			filepath.Join(versionDir, escapedVersion+ext),
			artifacts["module-0001"+ext],
			0600,
		); err != nil {
			t.Fatal(err)
		}
	}

	goMod := []byte(
		"module example.com/capture-test\n\n" +
			"go 1.26.0\n\n" +
			"require " + mod.Path + " " + mod.Version + "\n",
	)

	return goMod, goSum, cacheRoot
}

func TestReadGoModuleArtifactsForCaptureV3(t *testing.T) {
	goMod, goSum, cacheRoot := makeCaptureModuleCacheV3(t)

	mod, artifacts, err := ReadGoModuleArtifactsForCaptureV3(
		goMod,
		goSum,
		cacheRoot,
	)
	if err != nil {
		t.Fatalf("read module artifacts: %v", err)
	}

	if mod.Path != "example.com/buildfossil/fixture" {
		t.Fatalf("unexpected module path: %s", mod.Path)
	}

	if mod.Version != "v1.0.0" {
		t.Fatalf("unexpected module version: %s", mod.Version)
	}

	if len(artifacts) != 3 || len(mod.Artifacts) != 3 {
		t.Fatal("expected exactly three module artifacts")
	}

	if err := VerifyGoModuleArtifactsV3(
		mod, artifacts, goSum,
	); err != nil {
		t.Fatalf("returned artifacts failed verification: %v", err)
	}
}

func TestReadGoModuleArtifactsForCaptureV3RejectsInvalidDependencies(t *testing.T) {
	tests := []struct {
		name   string
		mutate func([]byte, []byte, string) ([]byte, []byte, string)
	}{
		{
			name: "no required module",
			mutate: func(_, goSum []byte, root string) ([]byte, []byte, string) {
				return []byte("module example.com/test\n\ngo 1.26.0\n"), goSum, root
			},
		},
		{
			name: "multiple required modules",
			mutate: func(goMod, goSum []byte, root string) ([]byte, []byte, string) {
				goMod = append(goMod,
					[]byte("require example.com/another v1.0.0\n")...)
				return goMod, goSum, root
			},
		},
		{
			name: "indirect dependency",
			mutate: func(goMod, goSum []byte, root string) ([]byte, []byte, string) {
				return []byte(strings.Replace(
					string(goMod), "v1.0.0\n", "v1.0.0 // indirect\n", 1,
				)), goSum, root
			},
		},
		{
			name: "missing cache",
			mutate: func(goMod, goSum []byte, _ string) ([]byte, []byte, string) {
				return goMod, goSum, filepath.Join(t.TempDir(), "missing")
			},
		},
		{
			name: "incorrect go.sum",
			mutate: func(goMod, _ []byte, root string) ([]byte, []byte, string) {
				return goMod, []byte("invalid go.sum\n"), root
			},
		},
		{
			name: "unsupported replace directive",
			mutate: func(goMod, goSum []byte, root string) ([]byte, []byte, string) {
				goMod = append(goMod,
					[]byte("replace example.com/buildfossil/fixture => ../local\n")...)
				return goMod, goSum, root
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			goMod, goSum, cacheRoot := makeCaptureModuleCacheV3(t)
			goMod, goSum, cacheRoot = tt.mutate(goMod, goSum, cacheRoot)

			if _, _, err := ReadGoModuleArtifactsForCaptureV3(
				goMod, goSum, cacheRoot,
			); err == nil {
				t.Fatal("invalid dependency configuration accepted")
			}
		})
	}
}

func TestReadGoModuleArtifactsForCaptureV3RejectsOversizedArtifact(t *testing.T) {
	goMod, goSum, cacheRoot := makeCaptureModuleCacheV3(t)

	artifactPath := filepath.Join(
		cacheRoot,
		"cache",
		"download",
		"example.com",
		"buildfossil",
		"fixture",
		"@v",
		"v1.0.0.zip",
	)

	oversized := bytes.Repeat(
		[]byte("X"),
		int(MaxGoModuleTotalSizeV3)+1,
	)

	if err := os.WriteFile(artifactPath, oversized, 0600); err != nil {
		t.Fatal(err)
	}

	_, _, err := ReadGoModuleArtifactsForCaptureV3(
		goMod,
		goSum,
		cacheRoot,
	)
	if err == nil {
		t.Fatal("oversized module artifact was accepted")
	}
}

func TestReadGoModuleArtifactsForCaptureV3RejectsSymlinkEscape(t *testing.T) {
	goMod, goSum, cacheRoot := makeCaptureModuleCacheV3(t)

	artifactPath := filepath.Join(
		cacheRoot,
		"cache",
		"download",
		"example.com",
		"buildfossil",
		"fixture",
		"@v",
		"v1.0.0.zip",
	)

	outsidePath := filepath.Join(t.TempDir(), "outside.zip")

	if err := os.WriteFile(
		outsidePath,
		[]byte("outside cache"),
		0600,
	); err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(artifactPath); err != nil {
		t.Fatal(err)
	}

	if err := os.Symlink(outsidePath, artifactPath); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	_, _, err := ReadGoModuleArtifactsForCaptureV3(
		goMod,
		goSum,
		cacheRoot,
	)
	if err == nil {
		t.Fatal("module cache symlink escape was accepted")
	}
}
