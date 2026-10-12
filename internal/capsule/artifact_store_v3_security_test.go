package capsule

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArtifactStoreV3RejectsReplacedContent(t *testing.T) {
	store, err := NewArtifactStoreV3(1024)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	name := "module-0001.zip"
	if err := store.Put(
		name, strings.NewReader("original"),
		8, artifactDigestV3("original"),
	); err != nil {
		t.Fatal(err)
	}

	replacement := filepath.Join(store.dir, ".replacement")
	if err := os.WriteFile(replacement, []byte("tampered"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, filepath.Join(store.dir, name)); err != nil {
		t.Fatal(err)
	}

	reader, err := store.Open(name)
	if err == nil {
		data, readErr := io.ReadAll(reader)
		reader.Close()
		if readErr == nil && string(data) == "tampered" {
			t.Fatal("SECURITY FAILURE: replaced content was returned")
		}
	}
}

func TestArtifactStoreV3RejectsSymlinkReplacement(t *testing.T) {
	store, err := NewArtifactStoreV3(1024)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	name := "module-0001.zip"
	if err := store.Put(
		name, strings.NewReader("original"),
		8, artifactDigestV3("original"),
	); err != nil {
		t.Fatal(err)
	}

	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte("outside"), 0600); err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(filepath.Join(store.dir, name)); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(store.dir, name)); err != nil {
		t.Fatal(err)
	}

	reader, err := store.Open(name)
	if err == nil {
		reader.Close()
		t.Fatal("SECURITY FAILURE: symlink replacement was opened")
	}
}

func TestArtifactStoreV3RecoversFromFailedPut(t *testing.T) {
	store, err := NewArtifactStoreV3(10)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if err := store.Put(
		"module.zip", strings.NewReader("bad"),
		3, artifactDigestV3("good"),
	); err == nil {
		t.Fatal("invalid digest accepted")
	}

	if store.Used() != 0 {
		t.Fatal("failed Put consumed budget")
	}

	if err := store.Put(
		"module.zip", strings.NewReader("good"),
		4, artifactDigestV3("good"),
	); err != nil {
		t.Fatalf("retry after failed Put: %v", err)
	}

	reader, err := store.Open("module.zip")
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()

	got, err := io.ReadAll(reader)
	if err != nil || string(got) != "good" {
		t.Fatalf("unexpected artifact: %q, %v", got, err)
	}
}

func TestArtifactStoreV3RejectsInPlaceMutation(t *testing.T) {
	store, err := NewArtifactStoreV3(1024)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	name := "module-0001.zip"
	if err := store.Put(
		name, strings.NewReader("original"),
		8, artifactDigestV3("original"),
	); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(
		filepath.Join(store.dir, name),
		[]byte("tampered"), 0600,
	); err != nil {
		t.Fatal(err)
	}

	reader, err := store.Open(name)
	if err == nil {
		reader.Close()
		t.Fatal("SECURITY FAILURE: in-place mutation was accepted")
	}
}

func TestArtifactStoreV3DoesNotOverwriteExistingFile(t *testing.T) {
	store, err := NewArtifactStoreV3(1024)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	name := "module-0001.zip"
	destination := filepath.Join(store.dir, name)

	if err := os.WriteFile(destination, []byte("existing"), 0600); err != nil {
		t.Fatal(err)
	}

	err = store.Put(
		name,
		strings.NewReader("verified"),
		8,
		artifactDigestV3("verified"),
	)
	if err == nil {
		t.Fatal("SECURITY FAILURE: existing destination overwritten")
	}

	data, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "existing" {
		t.Fatalf("existing file changed: %q", data)
	}

	if store.Used() != 0 {
		t.Fatal("failed publication consumed budget")
	}
}
