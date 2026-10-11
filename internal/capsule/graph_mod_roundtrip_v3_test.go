package capsule

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/mod/sumdb/dirhash"
)

func TestGraphModV3WriterReaderRoundTrip(t *testing.T) {
	manifest, files, artifacts := makeWorkspaceV3Fixture(t)

	const modulePath = "example.com/graph"
	const version = "v1.0.0"

	modData := []byte("module example.com/graph\n\ngo 1.23\n")

	hash, err := dirhash.Hash1(
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

	for i := range files {
		if files[i].Path == "go.sum" {
			files[i].Data = append(
				files[i].Data,
				[]byte(fmt.Sprintf(
					"%s %s/go.mod %s\n",
					modulePath, version, hash,
				))...,
			)
		}
	}

	manifest.GoGraphModFiles = []GoGraphModFileV3{
		{
			Path:    modulePath,
			Version: version,
			Artifact: GoModuleArtifact{
				Path:   "graph-mod-0001.mod",
				Size:   int64(len(modData)),
				SHA256: SHA256(modData),
			},
		},
	}

	artifacts["graph-mod-0001.mod"] = modData

	var archive bytes.Buffer

	if err := WriteWorkspaceV3(
		&archive, manifest, files, artifacts,
	); err != nil {
		t.Fatalf("write graph metadata capsule: %v", err)
	}

	filename := filepath.Join(t.TempDir(), "graph.bfc")
	if err := os.WriteFile(filename, archive.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}

	verified, err := ReadVerifiedV3(filename)
	if err != nil {
		t.Fatalf("read graph metadata capsule: %v", err)
	}

	if len(verified.Manifest.GoGraphModFiles) != 1 {
		t.Fatal("graph metadata missing from manifest")
	}

	if !bytes.Equal(
		verified.Artifacts["graph-mod-0001.mod"],
		modData,
	) {
		t.Fatal("graph module content changed")
	}

	verified.Artifacts["graph-mod-0001.mod"][0] ^= 0xff

	if err := VerifyGoGraphModFileV3(
		verified.Manifest.GoGraphModFiles[0],
		verified.Artifacts["graph-mod-0001.mod"],
		files[1].Data,
	); err == nil {
		t.Fatal("tampered graph metadata accepted")
	}
}
