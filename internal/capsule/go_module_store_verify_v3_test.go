package capsule

import (
	"bytes"
	"strings"
	"testing"
)

func storeModuleFixtureV3(
	t *testing.T,
	mod GoModuleV3,
	artifacts map[string][]byte,
) *ArtifactStoreV3 {
	t.Helper()

	store, err := NewArtifactStoreV3(MaxGoModuleTotalSizeV3)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("close artifact store: %v", err)
		}
	})

	for _, metadata := range mod.Artifacts {
		data, ok := artifacts[metadata.Path]
		if !ok {
			t.Fatalf("missing fixture artifact %q", metadata.Path)
		}

		if err := store.Put(
			metadata.Path,
			bytes.NewReader(data),
			int64(len(data)),
			strings.TrimPrefix(metadata.SHA256, "sha256:"),
		); err != nil {
			t.Fatalf("store artifact %s: %v", metadata.Path, err)
		}
	}
	return store
}

func fixtureGoSumV3(t *testing.T, verified VerifiedCapsuleV3) []byte {
	t.Helper()
	for _, file := range verified.Files {
		if file.Path == "go.sum" {
			return file.Data
		}
	}
	t.Fatal("fixture missing go.sum")
	return nil
}

func TestGoModuleFromStoreV3Valid(t *testing.T) {
	verified := makeVerifiedSnapshotFixture(t)
	if len(verified.Manifest.GoModules) != 1 {
		t.Fatalf(
			"expected one fixture module, got %d",
			len(verified.Manifest.GoModules),
		)
	}

	mod := verified.Manifest.GoModules[0]
	store := storeModuleFixtureV3(t, mod, verified.Artifacts)

	if err := verifyGoModuleFromStoreV3(
		mod, 1, fixtureGoSumV3(t, verified), store,
	); err != nil {
		t.Fatalf("file-based verification: %v", err)
	}
}

func TestGoModuleFromStoreV3RejectsMetadataMismatch(t *testing.T) {
	verified := makeVerifiedSnapshotFixture(t)
	mod := verified.Manifest.GoModules[0]
	store := storeModuleFixtureV3(t, mod, verified.Artifacts)
	mod.Artifacts = append([]GoModuleArtifact(nil), mod.Artifacts...)

	mod.Artifacts[0].SHA256 =
		"sha256:" + strings.Repeat("0", 64)

	if err := verifyGoModuleFromStoreV3(
		mod, 1, fixtureGoSumV3(t, verified), store,
	); err == nil {
		t.Fatal("changed manifest digest accepted")
	}
}

func TestGoModuleFromStoreV3RejectsWrongGoSum(t *testing.T) {
	verified := makeVerifiedSnapshotFixture(t)
	mod := verified.Manifest.GoModules[0]
	store := storeModuleFixtureV3(t, mod, verified.Artifacts)

	if err := verifyGoModuleFromStoreV3(
		mod, 1, []byte("invalid go.sum\n"), store,
	); err == nil {
		t.Fatal("invalid go.sum accepted")
	}
}

func TestGoModuleFromStoreV3RejectsWrongIndex(t *testing.T) {
	verified := makeVerifiedSnapshotFixture(t)
	mod := verified.Manifest.GoModules[0]
	store := storeModuleFixtureV3(t, mod, verified.Artifacts)

	if err := verifyGoModuleFromStoreV3(
		mod, 2, fixtureGoSumV3(t, verified), store,
	); err == nil {
		t.Fatal("wrong module index accepted")
	}
}

func TestGoModuleFromStoreV3RejectsNilStore(t *testing.T) {
	verified := makeVerifiedSnapshotFixture(t)
	if err := verifyGoModuleFromStoreV3(
		verified.Manifest.GoModules[0],
		1,
		fixtureGoSumV3(t, verified),
		nil,
	); err == nil {
		t.Fatal("nil artifact store accepted")
	}
}
