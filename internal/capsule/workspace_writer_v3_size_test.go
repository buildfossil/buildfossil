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

func TestWriteWorkspaceV3RejectsOversizedArchive(t *testing.T) {
	manifest, files, artifacts := makeWorkspaceV3Fixture(t)
	mod := &manifest.GoModules[0]

	// Add a valid, uncompressed 3 MiB file to the dependency ZIP.
	modData := artifacts["module-0001.mod"]

	var zipBuffer bytes.Buffer
	zw := zip.NewWriter(&zipBuffer)

	prefix := mod.Path + "@" + mod.Version + "/"

	for _, entry := range []struct {
		name string
		data []byte
	}{
		{"go.mod", modData},
		{"fixture.go", []byte("package fixture\nfunc Value() int { return 42 }\n")},
		{"padding.bin", bytes.Repeat([]byte("a"), 3<<20)},
	} {
		header := &zip.FileHeader{
			Name:   prefix + entry.name,
			Method: zip.Store,
		}
		header.SetMode(0644)

		w, err := zw.CreateHeader(header)
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

	artifacts["module-0001.zip"] = zipBuffer.Bytes()

	zipPath := filepath.Join(t.TempDir(), "module.zip")
	if err := os.WriteFile(zipPath, zipBuffer.Bytes(), 0600); err != nil {
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

	files[1].Data = []byte(fmt.Sprintf(
		"%s %s %s\n%s %s/go.mod %s\n",
		mod.Path, mod.Version, zipHash,
		mod.Path, mod.Version, modHash,
	))

	for i := range mod.Artifacts {
		name := mod.Artifacts[i].Path
		mod.Artifacts[i].Size = int64(len(artifacts[name]))
		mod.Artifacts[i].SHA256 = SHA256(artifacts[name])
	}

	// 9 MiB + 4 MiB + a small Go workspace.
	// Each file and the combined workspace stay within their own limits.
	files = append(files,
		WorkspaceFile{
			Path: "padding-a.txt",
			Data: bytes.Repeat([]byte("x"), 9<<20),
			Mode: 0600,
		},
		WorkspaceFile{
			Path: "padding-b.txt",
			Data: bytes.Repeat([]byte("y"), 4<<20),
			Mode: 0600,
		},
	)

	var output bytes.Buffer

	err = WriteWorkspaceV3(&output, manifest, files, artifacts)
	if err == nil {
		t.Fatal("oversized archive was accepted")
	}
	if !strings.Contains(err.Error(), "archive exceeds size limit") {
		t.Fatalf("wrong rejection reason: %v", err)
	}
	if output.Len() != 0 {
		t.Fatalf("writer emitted %d bytes before rejection", output.Len())
	}
}
