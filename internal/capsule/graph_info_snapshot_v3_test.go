package capsule

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"testing"
)

func TestGraphInfoSnapshotV3(t *testing.T) {
	manifest, files, artifacts := makeWorkspaceV3Fixture(t)

	full := manifest.GoModules[0]
	metadata := GoModuleIdentityV3{
		Path:    "example.com/metadata",
		Version: "v1.0.0",
	}

	manifest.GoSelectedModules = []GoModuleIdentityV3{
		{Path: full.Path, Version: full.Version},
		metadata,
	}

	if manifest.GoSelectedModules[0].Path >
		manifest.GoSelectedModules[1].Path {
		manifest.GoSelectedModules[0],
			manifest.GoSelectedModules[1] =
			manifest.GoSelectedModules[1],
			manifest.GoSelectedModules[0]
	}

	data := []byte(`{"Version":"v1.0.0"}`)
	manifest.GoGraphInfoFiles = []GoGraphInfoFileV3{
		{
			Path:    metadata.Path,
			Version: metadata.Version,
			Artifact: GoModuleArtifact{
				Path:   "graph-info-0001.info",
				Size:   int64(len(data)),
				SHA256: SHA256(data),
			},
		},
	}

	artifacts["graph-info-0001.info"] = data

	var archive bytes.Buffer
	if err := WriteWorkspaceV3(
		&archive, manifest, files, artifacts,
	); err != nil {
		t.Fatal(err)
	}

	// Use the verified reader rather than trusting the original inputs.
	capsulePath := t.TempDir() + "/capsule.bfc"
	if err := os.WriteFile(
		capsulePath, archive.Bytes(), 0600,
	); err != nil {
		t.Fatal(err)
	}

	verified, err := ReadVerifiedV3(capsulePath)
	if err != nil {
		t.Fatal(err)
	}

	snapshot, err := BuildGoModuleSnapshotV3(verified)
	if err != nil {
		t.Fatal(err)
	}

	expected := "cache/download/example.com/metadata/@v/v1.0.0.info"
	found := false

	reader := tar.NewReader(bytes.NewReader(snapshot))
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}

		if header.Name == expected {
			if found {
				t.Fatal("duplicate metadata cache entry")
			}
			found = true

			actual, err := io.ReadAll(reader)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(actual, data) {
				t.Fatal("metadata cache content mismatch")
			}
		}
	}

	if !found {
		t.Fatal("graph info missing from module snapshot")
	}
}
