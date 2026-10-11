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

func makeLegacyGoModuleZIPV3(
	t *testing.T,
	mod GoModuleV3,
	modData []byte,
	embeddedGoMods [][]byte,
) []byte {
	t.Helper()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	prefix := mod.Path + "@" + mod.Version + "/"

	for _, content := range embeddedGoMods {
		w, err := zw.Create(prefix + "go.mod")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(content); err != nil {
			t.Fatal(err)
		}
	}

	w, err := zw.Create(prefix + "fixture.go")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(
		"package fixture\nfunc Value() string { return \"offline-ok\" }\n",
	)); err != nil {
		t.Fatal(err)
	}

	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	return append([]byte(nil), buf.Bytes()...)
}

func TestVerifyGoModuleArtifactsV3AcceptsLegacyZIPWithoutGoMod(t *testing.T) {
	mod, artifacts, _ := makeGoModuleFixture(t)
	modData := artifacts["module-0001.mod"]

	zipData := makeLegacyGoModuleZIPV3(t, mod, modData, nil)
	artifacts["module-0001.zip"] = zipData

	for i := range mod.Artifacts {
		if mod.Artifacts[i].Path == "module-0001.zip" {
			mod.Artifacts[i].Size = int64(len(zipData))
			mod.Artifacts[i].SHA256 = SHA256(zipData)
		}
	}

	zipPath := filepath.Join(t.TempDir(), "legacy.zip")
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
				return nil, fmt.Errorf("unexpected file: %s", name)
			}
			return io.NopCloser(bytes.NewReader(modData)), nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	goSum := []byte(fmt.Sprintf(
		"%s %s %s\n%s %s/go.mod %s\n",
		mod.Path, mod.Version, zipHash,
		mod.Path, mod.Version, modHash,
	))

	if err := VerifyGoModuleArtifactsV3(
		mod, artifacts, goSum,
	); err != nil {
		t.Fatalf("valid legacy Go module rejected: %v", err)
	}
}

func TestVerifyEmbeddedGoModV3(t *testing.T) {
	mod, artifacts, _ := makeGoModuleFixture(t)
	modData := artifacts["module-0001.mod"]

	tests := []struct {
		name     string
		embedded [][]byte
		wantErr  string
	}{
		{
			name: "matching embedded go.mod",
			embedded: [][]byte{
				modData,
			},
		},
		{
			name: "missing embedded go.mod",
		},
		{
			name: "mismatched embedded go.mod",
			embedded: [][]byte{
				[]byte("module example.com/different\n"),
			},
			wantErr: "embedded go.mod",
		},
		{
			name: "duplicate embedded go.mod",
			embedded: [][]byte{
				modData,
				modData,
			},
			wantErr: "duplicate module go.mod",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			zipData := makeLegacyGoModuleZIPV3(
				t, mod, modData, tt.embedded,
			)

			zipPath := filepath.Join(t.TempDir(), "module.zip")
			if err := os.WriteFile(zipPath, zipData, 0600); err != nil {
				t.Fatal(err)
			}

			err := verifyEmbeddedGoMod(zipPath, mod, modData)

			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("valid ZIP rejected: %v", err)
				}
				return
			}

			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v; want %q", err, tt.wantErr)
			}
		})
	}
}

func TestVerifyGoModuleArtifactsV3RejectsMismatchedEmbeddedGoMod(t *testing.T) {
	mod, artifacts, _ := makeGoModuleFixture(t)
	modData := artifacts["module-0001.mod"]

	zipData := makeLegacyGoModuleZIPV3(
		t,
		mod,
		modData,
		[][]byte{
			[]byte("module example.com/different\n"),
		},
	)

	artifacts["module-0001.zip"] = zipData

	for i := range mod.Artifacts {
		if mod.Artifacts[i].Path == "module-0001.zip" {
			mod.Artifacts[i].Size = int64(len(zipData))
			mod.Artifacts[i].SHA256 = SHA256(zipData)
		}
	}

	zipPath := filepath.Join(t.TempDir(), "mismatch.zip")
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
			return io.NopCloser(bytes.NewReader(modData)), nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	goSum := []byte(fmt.Sprintf(
		"%s %s %s\n%s %s/go.mod %s\n",
		mod.Path, mod.Version, zipHash,
		mod.Path, mod.Version, modHash,
	))

	err = VerifyGoModuleArtifactsV3(mod, artifacts, goSum)
	if err == nil || !strings.Contains(err.Error(), "embedded go.mod") {
		t.Fatalf("error = %v; want embedded go.mod rejection", err)
	}
}
