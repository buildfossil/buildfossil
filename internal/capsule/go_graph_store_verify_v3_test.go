package capsule

import (
	"bytes"
	"fmt"
	"golang.org/x/mod/sumdb/dirhash"
	"io"
	"strings"
	"testing"
)

func TestGraphArtifactsFromStoreV3Empty(t *testing.T) {
	store, err := NewArtifactStoreV3(1024)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if err := verifyGoGraphArtifactsFromStoreV3(
		Manifest{}, nil, store,
	); err != nil {
		t.Fatal(err)
	}
}

func TestGraphArtifactsFromStoreV3InfoValid(t *testing.T) {
	graphFile, selected, data := validGraphInfoFixtureV3()

	store, err := NewArtifactStoreV3(1024)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if err := store.Put(
		graphFile.Artifact.Path,
		bytes.NewReader(data),
		int64(len(data)),
		strings.TrimPrefix(graphFile.Artifact.SHA256, "sha256:"),
	); err != nil {
		t.Fatal(err)
	}

	manifest := Manifest{
		GoSelectedModules: selected,
		GoGraphInfoFiles:  []GoGraphInfoFileV3{graphFile},
	}

	if err := verifyGoGraphArtifactsFromStoreV3(
		manifest, nil, store,
	); err != nil {
		t.Fatalf("valid graph info rejected: %v", err)
	}
}

func TestGraphArtifactsFromStoreV3RejectsInfoVersion(t *testing.T) {
	graphFile, selected, data := validGraphInfoFixtureV3()

	store, err := NewArtifactStoreV3(1024)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if err := store.Put(
		graphFile.Artifact.Path,
		bytes.NewReader(data),
		int64(len(data)),
		strings.TrimPrefix(graphFile.Artifact.SHA256, "sha256:"),
	); err != nil {
		t.Fatal(err)
	}

	graphFile.Version = "v1.1.0"

	manifest := Manifest{
		GoSelectedModules: selected,
		GoGraphInfoFiles:  []GoGraphInfoFileV3{graphFile},
	}

	if err := verifyGoGraphArtifactsFromStoreV3(
		manifest, nil, store,
	); err == nil {
		t.Fatal("wrong version accepted")
	}
}

func TestGraphArtifactsFromStoreV3RejectsTamperedDigest(t *testing.T) {
	graphFile, selected, data := validGraphInfoFixtureV3()

	store, err := NewArtifactStoreV3(1024)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if err := store.Put(
		graphFile.Artifact.Path,
		bytes.NewReader(data),
		int64(len(data)),
		strings.TrimPrefix(graphFile.Artifact.SHA256, "sha256:"),
	); err != nil {
		t.Fatal(err)
	}

	graphFile.Artifact.SHA256 =
		"sha256:" + strings.Repeat("0", 64)

	manifest := Manifest{
		GoSelectedModules: selected,
		GoGraphInfoFiles:  []GoGraphInfoFileV3{graphFile},
	}

	if err := verifyGoGraphArtifactsFromStoreV3(
		manifest, nil, store,
	); err == nil {
		t.Fatal("changed digest accepted")
	}
}

func TestGraphArtifactsFromStoreV3RejectsNilStore(t *testing.T) {
	if err := verifyGoGraphArtifactsFromStoreV3(
		Manifest{}, nil, nil,
	); err == nil {
		t.Fatal("nil store accepted")
	}
}

func TestGraphArtifactsFromStoreV3ModChecksum(t *testing.T) {
	const modulePath = "example.com/graph-dependency"
	const moduleVersion = "v1.2.3"

	modData := []byte(
		"module " + modulePath + "\n\ngo 1.26.0\n",
	)

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

	graphFile := GoGraphModFileV3{
		Path:    modulePath,
		Version: moduleVersion,
		Artifact: GoModuleArtifact{
			Path:   "graph-mod-0001.mod",
			Size:   int64(len(modData)),
			SHA256: SHA256(modData),
		},
	}

	store, err := NewArtifactStoreV3(MaxGoGraphModTotalSizeV3)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if err := store.Put(
		graphFile.Artifact.Path,
		bytes.NewReader(modData),
		int64(len(modData)),
		strings.TrimPrefix(graphFile.Artifact.SHA256, "sha256:"),
	); err != nil {
		t.Fatal(err)
	}

	manifest := Manifest{
		GoGraphModFiles: []GoGraphModFileV3{graphFile},
	}

	validGoSum := []byte(fmt.Sprintf(
		"%s %s/go.mod %s\n",
		modulePath, moduleVersion, modHash,
	))

	if err := verifyGoGraphArtifactsFromStoreV3(
		manifest, validGoSum, store,
	); err != nil {
		t.Fatalf("valid graph mod rejected: %v", err)
	}

	if err := verifyGoGraphArtifactsFromStoreV3(
		manifest, []byte("invalid go.sum\n"), store,
	); err == nil {
		t.Fatal("incorrect graph go.sum accepted")
	}
}
