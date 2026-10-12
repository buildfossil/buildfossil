package capsule

import (
	"bytes"
	"strings"
	"testing"
)

func completeStoreFixtureV3(
	t *testing.T,
	verified VerifiedCapsuleV3,
) *ArtifactStoreV3 {
	t.Helper()

	store, err := NewArtifactStoreV3(
		MaxGoModuleTotalSizeV3 +
			MaxGoGraphModTotalSizeV3 +
			MaxGoGraphInfoTotalSizeV3,
	)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("close store: %v", err)
		}
	})

	for _, mod := range verified.Manifest.GoModules {
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
	}

	for _, graph := range verified.Manifest.GoGraphModFiles {
		metadata := graph.Artifact
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

	for _, graph := range verified.Manifest.GoGraphInfoFiles {
		metadata := graph.Artifact
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

	return store
}

func TestCompleteGoArtifactStoreV3Valid(t *testing.T) {
	verified := makeVerifiedSnapshotFixture(t)
	store := completeStoreFixtureV3(t, verified)

	if err := verifyCompleteGoArtifactStoreV3(
		verified.Manifest,
		fixtureGoSumV3(t, verified),
		store,
	); err != nil {
		t.Fatalf("valid artifact set rejected: %v", err)
	}
}

func TestCompleteGoArtifactStoreV3RejectsExtra(t *testing.T) {
	verified := makeVerifiedSnapshotFixture(t)
	store := completeStoreFixtureV3(t, verified)

	data := []byte("unexpected")
	if err := store.Put(
		"unknown.info",
		bytes.NewReader(data),
		int64(len(data)),
		artifactDigestV3(string(data)),
	); err != nil {
		t.Fatal(err)
	}

	if err := verifyCompleteGoArtifactStoreV3(
		verified.Manifest,
		fixtureGoSumV3(t, verified),
		store,
	); err == nil {
		t.Fatal("extra artifact accepted")
	}
}

func TestCompleteGoArtifactStoreV3RejectsMissing(t *testing.T) {
	verified := makeVerifiedSnapshotFixture(t)

	store, err := NewArtifactStoreV3(MaxGoModuleTotalSizeV3)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if err := verifyCompleteGoArtifactStoreV3(
		verified.Manifest,
		fixtureGoSumV3(t, verified),
		store,
	); err == nil {
		t.Fatal("missing artifacts accepted")
	}
}

func TestCompleteGoArtifactStoreV3RejectsNilStore(t *testing.T) {
	verified := makeVerifiedSnapshotFixture(t)

	if err := verifyCompleteGoArtifactStoreV3(
		verified.Manifest,
		fixtureGoSumV3(t, verified),
		nil,
	); err == nil {
		t.Fatal("nil store accepted")
	}
}
