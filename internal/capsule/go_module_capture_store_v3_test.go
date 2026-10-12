package capsule

import (
	"bytes"
	"io"
	"reflect"
	"testing"
)

func TestReadGoModuleArtifactSetToStoreV3MatchesLegacy(
	t *testing.T,
) {
	goMod, goSum, cacheRoot := makeCaptureModuleCacheV3(t)

	// Exercise the existing fixture through its public reader,
	// avoiding manually constructed ZIP metadata.
	mod, artifacts, err := ReadGoModuleArtifactsForCaptureV3(
		goMod, goSum, cacheRoot,
	)
	if err != nil {
		t.Fatal(err)
	}

	store, err := NewArtifactStoreV3(MaxGoModuleTotalSizeV3)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	stored, err := ReadGoModuleArtifactSetToStoreV3(
		[]GoModuleIdentityV3{
			{Path: mod.Path, Version: mod.Version},
		},
		goSum, cacheRoot, store,
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(stored) != 1 || !reflect.DeepEqual(stored[0], mod) {
		t.Fatal("stored module metadata differs from legacy")
	}

	for name, expected := range artifacts {
		f, err := store.Open(name)
		if err != nil {
			t.Fatal(err)
		}
		actual, readErr := io.ReadAll(f)
		closeErr := f.Close()
		if readErr != nil || closeErr != nil {
			t.Fatalf("read %s: %v, %v", name, readErr, closeErr)
		}
		if !bytes.Equal(actual, expected) {
			t.Fatalf("stored artifact differs: %s", name)
		}
	}
}

func TestReadGoModuleArtifactSetToStoreV3RejectsNilStore(
	t *testing.T,
) {
	if _, err := ReadGoModuleArtifactSetToStoreV3(
		nil, nil, t.TempDir(), nil,
	); err == nil {
		t.Fatal("nil store accepted")
	}
}

func TestReadGoModuleArtifactSetToStoreV3Empty(
	t *testing.T,
) {
	store, err := NewArtifactStoreV3(MaxGoModuleTotalSizeV3)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	modules, err := ReadGoModuleArtifactSetToStoreV3(
		nil, nil, t.TempDir(), store,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(modules) != 0 || store.Used() != 0 {
		t.Fatal("empty module set unexpectedly stored artifacts")
	}
}
