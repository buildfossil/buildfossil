package capsule

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestGraphInfoRoundTripV3(t *testing.T) {
	manifest, files, artifacts := makeWorkspaceV3Fixture(t)

	full := manifest.GoModules[0]

	metadataModule := GoModuleIdentityV3{
		Path:    "example.com/metadata",
		Version: "v1.0.0",
	}

	manifest.GoSelectedModules = []GoModuleIdentityV3{
		metadataModule,
	}

	manifest.GoSelectedModules = append(
		manifest.GoSelectedModules,
		GoModuleIdentityV3{
			Path:    full.Path,
			Version: full.Version,
		},
	)

	// Ensure selected identities satisfy canonical path sorting.
	if manifest.GoSelectedModules[0].Path >
		manifest.GoSelectedModules[1].Path {
		manifest.GoSelectedModules[0],
			manifest.GoSelectedModules[1] =
			manifest.GoSelectedModules[1],
			manifest.GoSelectedModules[0]
	}

	infoData := []byte(`{"Version":"v1.0.0"}`)

	info := GoGraphInfoFileV3{
		Path:    metadataModule.Path,
		Version: metadataModule.Version,
		Artifact: GoModuleArtifact{
			Path:   "graph-info-0001.info",
			Size:   int64(len(infoData)),
			SHA256: SHA256(infoData),
		},
	}

	manifest.GoGraphInfoFiles = []GoGraphInfoFileV3{info}
	artifacts[info.Artifact.Path] = infoData

	var output bytes.Buffer
	if err := WriteWorkspaceV3(
		&output, manifest, files, artifacts,
	); err != nil {
		t.Fatalf("write extended capsule: %v", err)
	}

	filename := filepath.Join(t.TempDir(), "extended.bfc")

	if err := os.WriteFile(
		filename, output.Bytes(), 0600,
	); err != nil {
		t.Fatal(err)
	}

	verified, err := ReadVerifiedV3(filename)
	if err != nil {
		t.Fatalf("read extended capsule: %v", err)
	}

	if len(verified.Manifest.GoSelectedModules) != 2 {
		t.Fatal("selected graph not preserved")
	}

	if len(verified.Manifest.GoGraphInfoFiles) != 1 {
		t.Fatal("metadata-only module not preserved")
	}

	if !bytes.Equal(
		verified.Artifacts[info.Artifact.Path],
		infoData,
	) {
		t.Fatal("graph info contents changed")
	}
}
