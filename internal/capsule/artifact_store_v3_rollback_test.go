package capsule

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArtifactStoreV3RollbackAfterPublicationInspectionFailure(t *testing.T) {
	store, err := NewArtifactStoreV3(1024)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	name := "module-0001.zip"
	destination := filepath.Join(store.dir, name)

	// Force inspection to fail after os.Link succeeds.
	if err := store.root.Close(); err != nil {
		t.Fatal(err)
	}

	err = store.Put(
		name,
		strings.NewReader("verified"),
		8,
		artifactDigestV3("verified"),
	)
	if err == nil {
		t.Fatal("expected post-publication inspection failure")
	}

	if _, statErr := os.Lstat(destination); !os.IsNotExist(statErr) {
		t.Fatalf("published artifact survived failed Put: %v", statErr)
	}

	if store.Used() != 0 {
		t.Fatalf("failed Put consumed budget: %d", store.Used())
	}

	if len(store.names) != 0 {
		t.Fatal("failed Put registered an artifact")
	}

	// Restore the directory handle to verify that retry still works.
	store.root, err = os.OpenRoot(store.dir)
	if err != nil {
		t.Fatal(err)
	}

	if err := store.Put(
		name,
		strings.NewReader("verified"),
		8,
		artifactDigestV3("verified"),
	); err != nil {
		t.Fatalf("retry after rollback: %v", err)
	}

	reader, err := store.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
}
