package capsule

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/mod/sumdb/dirhash"
)

func TestGoModuleTotalUnpackedBudgetIntegrationV3(t *testing.T) {
	if testing.Short() {
		t.Skip("large ZIP verification integration test")
	}

	const (
		moduleCount = 6
		moduleMiB   = 43
		version     = "v1.0.0"
	)

	var modules []GoModuleV3
	artifacts := make(map[string][]byte)
	var goSum []byte

	chunk := bytes.Repeat([]byte("x"), 1<<20)

	for i := 1; i <= moduleCount; i++ {
		modulePath := fmt.Sprintf("example.com/rew-budget-%d", i)
		prefix := fmt.Sprintf("module-%04d", i)
		modData := []byte("module " + modulePath + "\n\ngo 1.26.0\n")

		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)

		for _, entry := range []struct {
			name string
			data []byte
		}{
			{modulePath + "@" + version + "/go.mod", modData},
		} {
			w, err := zw.Create(entry.name)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := w.Write(entry.data); err != nil {
				t.Fatal(err)
			}
		}

		w, err := zw.Create(modulePath + "@" + version + "/data.bin")
		if err != nil {
			t.Fatal(err)
		}
		for j := 0; j < moduleMiB; j++ {
			if _, err := w.Write(chunk); err != nil {
				t.Fatal(err)
			}
		}
		if err := zw.Close(); err != nil {
			t.Fatal(err)
		}

		zipData := append([]byte(nil), buf.Bytes()...)
		zipPath := filepath.Join(t.TempDir(), "module.zip")
		if err := os.WriteFile(zipPath, zipData, 0600); err != nil {
			t.Fatal(err)
		}
		zipHash, err := dirhash.HashZip(zipPath, dirhash.Hash1)
		if err != nil {
			t.Fatal(err)
		}

		modHash, err := dirhash.Hash1(
			[]string{"go.mod"},
			func(name string) (io.ReadCloser, error) {
				if name != "go.mod" {
					return nil, fmt.Errorf("unexpected name %q", name)
				}
				return io.NopCloser(bytes.NewReader(modData)), nil
			},
		)
		if err != nil {
			t.Fatal(err)
		}

		goSum = fmt.Appendf(
			goSum,
			"%s %s %s\n%s %s/go.mod %s\n",
			modulePath, version, zipHash,
			modulePath, version, modHash,
		)

		parts := map[string][]byte{
			prefix + ".zip":  zipData,
			prefix + ".mod":  modData,
			prefix + ".info": []byte(`{"Version":"v1.0.0"}`),
		}

		mod := GoModuleV3{Path: modulePath, Version: version}
		for _, suffix := range []string{".zip", ".mod", ".info"} {
			name := prefix + suffix
			data := parts[name]
			artifacts[name] = data
			mod.Artifacts = append(mod.Artifacts, GoModuleArtifact{
				Path:   name,
				Size:   int64(len(data)),
				SHA256: SHA256(data),
			})
		}
		modules = append(modules, mod)
	}

	store, err := NewArtifactStoreV3(MaxGoModuleTotalSizeV3)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("close store: %v", err)
		}
	})

	for _, mod := range modules {
		for _, metadata := range mod.Artifacts {
			data := artifacts[metadata.Path]
			if err := store.Put(
				metadata.Path,
				bytes.NewReader(data),
				int64(len(data)),
				strings.TrimPrefix(metadata.SHA256, "sha256:"),
			); err != nil {
				t.Fatal(err)
			}
		}
	}

	firstFive := modules[:5]
	firstFiveArtifacts := make(map[string][]byte)
	for _, mod := range firstFive {
		for _, metadata := range mod.Artifacts {
			firstFiveArtifacts[metadata.Path] = artifacts[metadata.Path]
		}
	}

	t.Run("memory within budget", func(t *testing.T) {
		if err := VerifyGoModuleArtifactSetV3(
			firstFive, firstFiveArtifacts, goSum,
		); err != nil {
			t.Fatalf("valid set rejected: %v", err)
		}
	})

	t.Run("memory exceeds budget", func(t *testing.T) {
		err := VerifyGoModuleArtifactSetV3(modules, artifacts, goSum)
		if err == nil || !strings.Contains(
			err.Error(), "total Go module ZIP unpacked size exceeded",
		) {
			t.Fatalf("expected aggregate limit error, got %v", err)
		}
	})

	t.Run("store within budget", func(t *testing.T) {
		if err := verifyGoModuleSubsetFromStoreV3(
			firstFive, goSum, store,
		); err != nil {
			t.Fatalf("valid stored set rejected: %v", err)
		}
	})

	t.Run("store exceeds budget", func(t *testing.T) {
		err := verifyGoModuleSubsetFromStoreV3(modules, goSum, store)
		if err == nil || !strings.Contains(
			err.Error(), "total Go module ZIP unpacked size exceeded",
		) {
			t.Fatalf("expected aggregate limit error, got %v", err)
		}
	})
}
