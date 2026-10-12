package capsule

import (
	"bytes"
	"golang.org/x/mod/sumdb/dirhash"
	"os"
	"path/filepath"
	"testing"
)

func TestVerifyGoModuleZipFileV3RejectsInvalidZIP(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid.zip")
	if err := os.WriteFile(path, []byte("not a zip"), 0600); err != nil {
		t.Fatal(err)
	}

	mod := GoModuleV3{
		Path:    "example.com/example",
		Version: "v1.0.0",
	}

	if _, err := verifyGoModuleZipFileV3(path, mod, nil); err == nil {
		t.Fatal("invalid ZIP accepted")
	}
}

func TestVerifyGoModuleZipFileV3RejectsMissingZIP(t *testing.T) {
	mod := GoModuleV3{
		Path:    "example.com/example",
		Version: "v1.0.0",
	}

	if _, err := verifyGoModuleZipFileV3(
		filepath.Join(t.TempDir(), "missing.zip"),
		mod,
		nil,
	); err == nil {
		t.Fatal("missing ZIP accepted")
	}
}

func TestVerifyGoModuleZipFileV3ValidFixture(t *testing.T) {
	mod, artifacts, goSum := makeGoModuleFixture(t)

	zipPath := filepath.Join(t.TempDir(), "module.zip")
	if err := os.WriteFile(
		zipPath, artifacts["module-0001.zip"], 0600,
	); err != nil {
		t.Fatal(err)
	}

	got, err := verifyGoModuleZipFileV3(
		zipPath, mod, artifacts["module-0001.mod"],
	)
	if err != nil {
		t.Fatalf("valid module ZIP rejected: %v", err)
	}

	expected, err := dirhash.HashZip(zipPath, dirhash.Hash1)
	if err != nil {
		t.Fatal(err)
	}

	if got != expected {
		t.Fatalf("ZIP hash mismatch: got %q, want %q", got, expected)
	}

	if !bytes.Contains(goSum, []byte(got)) {
		t.Fatal("ZIP checksum absent from fixture go.sum")
	}

	if err := VerifyGoModuleArtifactsV3(
		mod, artifacts, goSum,
	); err != nil {
		t.Fatalf("legacy verifier rejected same ZIP: %v", err)
	}
}

func TestVerifyGoModuleZipFileV3RejectsEmbeddedGoModMismatch(t *testing.T) {
	mod, artifacts, _ := makeGoModuleFixture(t)

	zipPath := filepath.Join(t.TempDir(), "module.zip")
	if err := os.WriteFile(
		zipPath, artifacts["module-0001.zip"], 0600,
	); err != nil {
		t.Fatal(err)
	}

	modifiedMod := []byte(
		"module example.com/buildfossil/fixture\n\ngo 1.25.0\n",
	)

	if _, err := verifyGoModuleZipFileV3(
		zipPath, mod, modifiedMod,
	); err == nil {
		t.Fatal("mismatched embedded go.mod accepted")
	}
}

func TestVerifyGoModuleZipFileV3RejectsCorruptedFixture(t *testing.T) {
	mod, artifacts, _ := makeGoModuleFixture(t)

	corrupted := append(
		[]byte(nil), artifacts["module-0001.zip"]...,
	)
	corrupted[0] ^= 0xff

	zipPath := filepath.Join(t.TempDir(), "corrupted.zip")
	if err := os.WriteFile(zipPath, corrupted, 0600); err != nil {
		t.Fatal(err)
	}

	if _, err := verifyGoModuleZipFileV3(
		zipPath, mod, artifacts["module-0001.mod"],
	); err == nil {
		t.Fatal("corrupted module ZIP accepted")
	}
}
