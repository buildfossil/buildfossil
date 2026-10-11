package capsule

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/mod/sumdb/dirhash"
)

func TestGraphModV3SnapshotIntegrity(t *testing.T) {
	manifest, files, artifacts := makeWorkspaceV3Fixture(t)

	modData := []byte("module example.com/graph\n\ngo 1.23\n")

	modHash, err := dirhash.Hash1(
		[]string{"go.mod"},
		func(name string) (io.ReadCloser, error) {
			if name != "go.mod" {
				return nil, fmt.Errorf("unexpected file %q", name)
			}
			return io.NopCloser(bytes.NewReader(modData)), nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	for i := range files {
		if files[i].Path == "go.sum" {
			files[i].Data = append(
				files[i].Data,
				[]byte(fmt.Sprintf(
					"example.com/graph v1.0.0/go.mod %s\n",
					modHash,
				))...,
			)
		}
	}

	manifest.GoGraphModFiles = []GoGraphModFileV3{
		{
			Path:    "example.com/graph",
			Version: "v1.0.0",
			Artifact: GoModuleArtifact{
				Path:   "graph-mod-0001.mod",
				Size:   int64(len(modData)),
				SHA256: SHA256(modData),
			},
		},
	}

	artifacts["graph-mod-0001.mod"] = append([]byte(nil), modData...)

	var archive bytes.Buffer
	if err := WriteWorkspaceV3(
		&archive, manifest, files, artifacts,
	); err != nil {
		t.Fatal(err)
	}

	capsulePath := filepath.Join(t.TempDir(), "graph-snapshot.bfc")
	if err := os.WriteFile(capsulePath, archive.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}

	verified, err := ReadVerifiedV3(capsulePath)
	if err != nil {
		t.Fatalf("read verified graph capsule: %v", err)
	}

	t.Run("valid snapshot contains graph metadata", func(t *testing.T) {
		snapshot, err := BuildGoModuleSnapshotV3(verified)
		if err != nil {
			t.Fatal(err)
		}

		tr := tar.NewReader(bytes.NewReader(snapshot))
		found := false

		for {
			header, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}

			if header.Name == "cache/download/example.com/graph/@v/v1.0.0.mod" {
				data, err := io.ReadAll(tr)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(data, modData) {
					t.Fatal("incorrect graph mod contents")
				}
				found = true
			}
		}

		if !found {
			t.Fatal("graph mod missing from Docker snapshot")
		}
	})

	t.Run("reject changed bytes", func(t *testing.T) {
		mutated := verified
		mutated.Artifacts = make(map[string][]byte, len(artifacts))
		for name, data := range artifacts {
			mutated.Artifacts[name] = append([]byte(nil), data...)
		}

		mutated.Artifacts["graph-mod-0001.mod"][0] ^= 0xff

		if _, err := BuildGoModuleSnapshotV3(mutated); err == nil {
			t.Fatal("tampered graph mod accepted")
		}
	})

	t.Run("reject changed checksum", func(t *testing.T) {
		mutated := verified
		mutated.Manifest.GoGraphModFiles =
			append([]GoGraphModFileV3(nil), manifest.GoGraphModFiles...)

		mutated.Manifest.GoGraphModFiles[0].Artifact.SHA256 =
			SHA256([]byte("different"))

		if _, err := BuildGoModuleSnapshotV3(mutated); err == nil {
			t.Fatal("incorrect graph checksum accepted")
		}
	})
}
