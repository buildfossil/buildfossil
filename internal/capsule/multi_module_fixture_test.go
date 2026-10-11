package capsule

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/mod/sumdb/dirhash"
)

func makeIndexedGoModuleFixture(
	t *testing.T,
	index int,
	modulePath string,
) (GoModuleV3, map[string][]byte, []byte) {
	t.Helper()

	const version = "v1.0.0"

	modData := []byte("module " + modulePath + "\n\ngo 1.26.0\n")
	sourceData := []byte("package fixture\n\nfunc Value() int { return 42 }\n")

	var buffer bytes.Buffer
	zw := zip.NewWriter(&buffer)

	for _, entry := range []struct {
		name string
		data []byte
	}{
		{modulePath + "@" + version + "/go.mod", modData},
		{modulePath + "@" + version + "/fixture.go", sourceData},
	} {
		w, err := zw.Create(entry.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(entry.data); err != nil {
			t.Fatal(err)
		}
	}

	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	prefix := fmt.Sprintf("module-%04d", index)

	artifacts := map[string][]byte{
		prefix + ".zip":  append([]byte(nil), buffer.Bytes()...),
		prefix + ".mod":  modData,
		prefix + ".info": []byte(`{"Version":"v1.0.0"}`),
	}

	zipPath := filepath.Join(t.TempDir(), "module.zip")
	if err := os.WriteFile(zipPath, artifacts[prefix+".zip"], 0600); err != nil {
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
		modulePath, version, zipHash,
		modulePath, version, modHash,
	))

	mod := GoModuleV3{
		Path:    modulePath,
		Version: version,
	}

	for _, ext := range []string{".zip", ".mod", ".info"} {
		name := prefix + ext
		data := artifacts[name]

		mod.Artifacts = append(mod.Artifacts, GoModuleArtifact{
			Path:   name,
			Size:   int64(len(data)),
			SHA256: SHA256(data),
		})
	}

	return mod, artifacts, goSum
}
