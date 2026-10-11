package capsule

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/mod/module"
)

func TestMultiModuleReaderRejectsMissingArtifact(t *testing.T) {
	mod, _, sum := makeIndexedGoModuleFixture(t, 1, "example.com/a")

	_, _, err := ReadGoModuleArtifactSetForCaptureV3(
		[]GoModuleIdentityV3{{Path: mod.Path, Version: mod.Version}},
		sum,
		t.TempDir(),
	)
	if err == nil || !strings.Contains(err.Error(), "missing cached module artifact") {
		t.Fatalf("expected missing artifact error, got %v", err)
	}
}

func TestMultiModuleReaderRejectsSymlinkEscape(t *testing.T) {
	mod, artifacts, sum := makeIndexedGoModuleFixture(t, 1, "example.com/a")

	cache := t.TempDir()
	outside := t.TempDir()

	escaped, err := module.EscapePath(mod.Path)
	if err != nil {
		t.Fatal(err)
	}

	dir := filepath.Join(cache, "cache", "download", escaped, "@v")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}

	for _, ext := range []string{".mod", ".info"} {
		if err := os.WriteFile(
			filepath.Join(dir, mod.Version+ext),
			artifacts["module-0001"+ext],
			0600,
		); err != nil {
			t.Fatal(err)
		}
	}

	outsideZIP := filepath.Join(outside, "module.zip")
	if err := os.WriteFile(outsideZIP, artifacts["module-0001.zip"], 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsideZIP, filepath.Join(dir, mod.Version+".zip")); err != nil {
		t.Fatal(err)
	}

	_, _, err = ReadGoModuleArtifactSetForCaptureV3(
		[]GoModuleIdentityV3{{Path: mod.Path, Version: mod.Version}},
		sum,
		cache,
	)
	if err == nil {
		t.Fatal("symlink escape accepted")
	}
}

func TestMultiModuleReaderRejectsUnsortedModules(t *testing.T) {
	_, _, err := ReadGoModuleArtifactSetForCaptureV3(
		[]GoModuleIdentityV3{
			{Path: "example.com/b", Version: "v1.0.0"},
			{Path: "example.com/a", Version: "v1.0.0"},
		},
		nil,
		t.TempDir(),
	)
	if err == nil {
		t.Fatal("unsorted module list accepted")
	}
}

func TestMultiModuleReaderRejectsOversizedArtifact(t *testing.T) {
	mod, _, sum := makeIndexedGoModuleFixture(t, 1, "example.com/a")
	cache := t.TempDir()

	dir := filepath.Join(cache, "cache", "download", mod.Path, "@v")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, mod.Version+".zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(MaxGoModuleTotalSizeV3 + 1); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	_, _, err = ReadGoModuleArtifactSetForCaptureV3(
		[]GoModuleIdentityV3{{Path: mod.Path, Version: mod.Version}},
		sum,
		cache,
	)
	if err == nil || !strings.Contains(err.Error(), "size") {
		t.Fatalf("expected size limit error, got %v", err)
	}
}

func TestMultiModuleReaderRejectsInternalSymlink(t *testing.T) {
	mod, artifacts, sum := makeIndexedGoModuleFixture(
		t, 1, "example.com/a",
	)

	cache := t.TempDir()

	escaped, err := module.EscapePath(mod.Path)
	if err != nil {
		t.Fatal(err)
	}

	dir := filepath.Join(cache, "cache", "download", escaped, "@v")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}

	for _, ext := range []string{".zip", ".mod", ".info"} {
		if err := os.WriteFile(
			filepath.Join(dir, mod.Version+ext),
			artifacts["module-0001"+ext],
			0600,
		); err != nil {
			t.Fatal(err)
		}
	}

	realZIP := filepath.Join(dir, "actual.zip")

	if err := os.Rename(
		filepath.Join(dir, mod.Version+".zip"),
		realZIP,
	); err != nil {
		t.Fatal(err)
	}

	if err := os.Symlink(
		"actual.zip",
		filepath.Join(dir, mod.Version+".zip"),
	); err != nil {
		t.Fatal(err)
	}

	_, _, err = ReadGoModuleArtifactSetForCaptureV3(
		[]GoModuleIdentityV3{
			{Path: mod.Path, Version: mod.Version},
		},
		sum,
		cache,
	)

	if err == nil {
		t.Fatal("internal symlink accepted")
	}
}
