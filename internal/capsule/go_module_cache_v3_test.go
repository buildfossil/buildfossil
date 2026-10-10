package capsule

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/mod/module"
)

func TestPrepareGoModuleCacheV3(t *testing.T) {
	verified := makeVerifiedCapsuleV3(t)

	root := t.TempDir()

	if err := PrepareGoModuleCacheV3(root, verified); err != nil {
		t.Fatalf("prepare module cache: %v", err)
	}

	mod := verified.Manifest.GoModules[0]

	escapedPath, err := module.EscapePath(mod.Path)
	if err != nil {
		t.Fatal(err)
	}

	escapedVersion, err := module.EscapeVersion(mod.Version)
	if err != nil {
		t.Fatal(err)
	}

	base := filepath.Join(
		root,
		"cache",
		"download",
		filepath.FromSlash(escapedPath),
		"@v",
	)

	for _, ext := range []string{".zip", ".mod", ".info"} {
		actual, err := os.ReadFile(
			filepath.Join(base, escapedVersion+ext),
		)
		if err != nil {
			t.Fatal(err)
		}

		expected := verified.Artifacts["module-0001"+ext]

		if !bytes.Equal(actual, expected) {
			t.Fatalf("cache artifact mismatch: %s", ext)
		}
	}
}

func TestPrepareGoModuleCacheV3RejectsCorruptedArtifact(t *testing.T) {
	verified := makeVerifiedCapsuleV3(t)
	verified.Artifacts["module-0001.mod"][0] ^= 0xff

	root := t.TempDir()

	if err := PrepareGoModuleCacheV3(root, verified); err == nil {
		t.Fatal("corrupted module artifact accepted")
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 0 {
		t.Fatal("cache was modified despite verification failure")
	}
}

func TestPrepareGoModuleCacheV3RejectsNonemptyRoot(t *testing.T) {
	verified := makeVerifiedCapsuleV3(t)

	root := t.TempDir()
	existing := filepath.Join(root, "existing.txt")

	if err := os.WriteFile(existing, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}

	if err := PrepareGoModuleCacheV3(root, verified); err == nil {
		t.Fatal("nonempty cache root accepted")
	}

	data, err := os.ReadFile(existing)
	if err != nil {
		t.Fatal(err)
	}

	if string(data) != "keep" {
		t.Fatal("existing file was changed")
	}
}
