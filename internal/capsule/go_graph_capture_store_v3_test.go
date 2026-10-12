package capsule

import (
	"bytes"
	"strings"
	"testing"
)

func graphCaptureStoreFixtureV3(
	t *testing.T,
) (
	Manifest,
	[]byte,
	map[string][]byte,
	map[string][]byte,
	*ArtifactStoreV3,
) {
	t.Helper()

	manifest, files, artifacts :=
		makeExtendedSeventeenModuleWorkspaceV3(t)

	var goSum []byte
	for _, file := range files {
		if file.Path == "go.sum" {
			goSum = file.Data
		}
	}

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

	for _, mod := range manifest.GoModules {
		for _, artifact := range mod.Artifacts {
			data := artifacts[artifact.Path]
			if err := store.Put(
				artifact.Path,
				bytes.NewReader(data),
				artifact.Size,
				strings.TrimPrefix(artifact.SHA256, "sha256:"),
			); err != nil {
				t.Fatal(err)
			}
		}
	}

	graphMods := make(map[string][]byte)
	graphInfos := make(map[string][]byte)

	for _, graph := range manifest.GoGraphModFiles {
		graphMods[graph.Artifact.Path] =
			artifacts[graph.Artifact.Path]
	}
	for _, graph := range manifest.GoGraphInfoFiles {
		graphInfos[graph.Artifact.Path] =
			artifacts[graph.Artifact.Path]
	}

	return manifest, goSum, graphMods, graphInfos, store
}

func TestAddGoGraphArtifactsToStoreV3CompleteSet(t *testing.T) {
	manifest, goSum, graphMods, graphInfos, store :=
		graphCaptureStoreFixtureV3(t)

	if err := AddGoGraphArtifactsToStoreV3(
		manifest, goSum, graphMods, graphInfos, store,
	); err != nil {
		t.Fatal(err)
	}

	if len(store.names) != 39 {
		t.Fatalf("expected 39 artifacts, got %d", len(store.names))
	}

	if err := verifyCompleteGoArtifactStoreV3(
		manifest, goSum, store,
	); err != nil {
		t.Fatalf("complete stored set rejected: %v", err)
	}
}

func TestAddGoGraphArtifactsToStoreV3RejectsMissing(
	t *testing.T,
) {
	manifest, goSum, graphMods, graphInfos, store :=
		graphCaptureStoreFixtureV3(t)

	delete(graphMods, manifest.GoGraphModFiles[0].Artifact.Path)
	before := len(store.names)

	if err := AddGoGraphArtifactsToStoreV3(
		manifest, goSum, graphMods, graphInfos, store,
	); err == nil {
		t.Fatal("missing graph mod accepted")
	}

	if len(store.names) != before {
		t.Fatal("invalid graph set modified store")
	}
}

func TestAddGoGraphArtifactsToStoreV3RejectsTampering(
	t *testing.T,
) {
	manifest, goSum, graphMods, graphInfos, store :=
		graphCaptureStoreFixtureV3(t)

	name := manifest.GoGraphInfoFiles[0].Artifact.Path
	graphInfos[name] = []byte(`{"Version":"v9.9.9"}`)
	before := len(store.names)

	if err := AddGoGraphArtifactsToStoreV3(
		manifest, goSum, graphMods, graphInfos, store,
	); err == nil {
		t.Fatal("tampered graph info accepted")
	}

	if len(store.names) != before {
		t.Fatal("tampered graph set modified store")
	}
}

func TestAddGoGraphArtifactsToStoreV3RejectsCollision(
	t *testing.T,
) {
	manifest, goSum, graphMods, graphInfos, store :=
		graphCaptureStoreFixtureV3(t)

	graph := manifest.GoGraphModFiles[0]
	data := graphMods[graph.Artifact.Path]

	if err := store.Put(
		graph.Artifact.Path,
		bytes.NewReader(data),
		graph.Artifact.Size,
		strings.TrimPrefix(graph.Artifact.SHA256, "sha256:"),
	); err != nil {
		t.Fatal(err)
	}

	before := len(store.names)
	if err := AddGoGraphArtifactsToStoreV3(
		manifest, goSum, graphMods, graphInfos, store,
	); err == nil {
		t.Fatal("stored graph name collision accepted")
	}
	if len(store.names) != before {
		t.Fatal("collision modified store")
	}
}

func TestAddGoGraphArtifactsToStoreV3RejectsNilStore(
	t *testing.T,
) {
	if err := AddGoGraphArtifactsToStoreV3(
		Manifest{}, nil, nil, nil, nil,
	); err == nil {
		t.Fatal("nil store accepted")
	}
}
