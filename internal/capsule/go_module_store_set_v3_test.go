package capsule

import (
	"bytes"
	"strings"
	"testing"
)

func TestGoModuleSetFromStoreV3Valid(t *testing.T) {
	verified := makeVerifiedSnapshotFixture(t)
	mod := verified.Manifest.GoModules[0]
	store := storeModuleFixtureV3(t, mod, verified.Artifacts)

	if err := verifyGoModuleSetFromStoreV3(
		verified.Manifest.GoModules,
		fixtureGoSumV3(t, verified),
		store,
	); err != nil {
		t.Fatalf("verify module set: %v", err)
	}
}

func TestGoModuleSetFromStoreV3RejectsExtraArtifact(t *testing.T) {
	verified := makeVerifiedSnapshotFixture(t)
	mod := verified.Manifest.GoModules[0]

	store, err := NewArtifactStoreV3(MaxGoModuleTotalSizeV3)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	for _, metadata := range mod.Artifacts {
		data := verified.Artifacts[metadata.Path]
		if err := store.Put(
			metadata.Path,
			bytes.NewReader(data),
			int64(len(data)),
			strings.TrimPrefix(metadata.SHA256, "sha256:"),
		); err != nil {
			t.Fatal(err)
		}
	}

	extra := []byte("extra")
	if err := store.Put(
		"unexpected.info",
		bytes.NewReader(extra),
		int64(len(extra)),
		artifactDigestV3(string(extra)),
	); err != nil {
		t.Fatal(err)
	}

	if err := verifyGoModuleSetFromStoreV3(
		verified.Manifest.GoModules,
		fixtureGoSumV3(t, verified),
		store,
	); err == nil {
		t.Fatal("extra artifact accepted")
	}
}

func TestGoModuleSetFromStoreV3RejectsMissingArtifact(t *testing.T) {
	verified := makeVerifiedSnapshotFixture(t)
	store, err := NewArtifactStoreV3(MaxGoModuleTotalSizeV3)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if err := verifyGoModuleSetFromStoreV3(
		verified.Manifest.GoModules,
		fixtureGoSumV3(t, verified),
		store,
	); err == nil {
		t.Fatal("missing artifacts accepted")
	}
}

func TestGoModuleSetFromStoreV3RejectsInvalidOrder(t *testing.T) {
	verified := makeVerifiedSnapshotFixture(t)
	mod := verified.Manifest.GoModules[0]
	store := storeModuleFixtureV3(t, mod, verified.Artifacts)

	modules := []GoModuleV3{mod, mod}
	if err := verifyGoModuleSetFromStoreV3(
		modules,
		fixtureGoSumV3(t, verified),
		store,
	); err == nil {
		t.Fatal("duplicate modules accepted")
	}
}

func TestGoModuleSetFromStoreV3RejectsNilStore(t *testing.T) {
	verified := makeVerifiedSnapshotFixture(t)

	if err := verifyGoModuleSetFromStoreV3(
		verified.Manifest.GoModules,
		fixtureGoSumV3(t, verified),
		nil,
	); err == nil {
		t.Fatal("nil store accepted")
	}
}

func TestGoModuleSubsetFromStoreV3AllowsSeparateGraphArtifact(t *testing.T) {
	verified := makeVerifiedSnapshotFixture(t)
	mod := verified.Manifest.GoModules[0]

	store, err := NewArtifactStoreV3(MaxGoModuleTotalSizeV3)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	for _, metadata := range mod.Artifacts {
		data := verified.Artifacts[metadata.Path]

		if err := store.Put(
			metadata.Path,
			bytes.NewReader(data),
			int64(len(data)),
			strings.TrimPrefix(metadata.SHA256, "sha256:"),
		); err != nil {
			t.Fatal(err)
		}
	}

	graphData := []byte("module example.com/graph\n")

	if err := store.Put(
		"graph-0001.mod",
		bytes.NewReader(graphData),
		int64(len(graphData)),
		artifactDigestV3(string(graphData)),
	); err != nil {
		t.Fatal(err)
	}

	if err := verifyGoModuleSubsetFromStoreV3(
		verified.Manifest.GoModules,
		fixtureGoSumV3(t, verified),
		store,
	); err != nil {
		t.Fatalf("selected subset rejected: %v", err)
	}

	if err := verifyGoModuleSetFromStoreV3(
		verified.Manifest.GoModules,
		fixtureGoSumV3(t, verified),
		store,
	); err == nil {
		t.Fatal("strict verifier accepted extra artifact")
	}
}
