package capsule

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArtifactStoreV3PrivatePermissions(t *testing.T) {
	store, err := NewArtifactStoreV3(1024)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	dirInfo, err := os.Stat(store.dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := dirInfo.Mode().Perm(); got != 0700 {
		t.Fatalf("directory permissions = %#o, want 0700", got)
	}

	name := "module-0001.zip"
	if err := store.Put(
		name, strings.NewReader("verified"),
		8, artifactDigestV3("verified"),
	); err != nil {
		t.Fatal(err)
	}

	fileInfo, err := os.Stat(filepath.Join(store.dir, name))
	if err != nil {
		t.Fatal(err)
	}
	if got := fileInfo.Mode().Perm(); got != 0600 {
		t.Fatalf("artifact permissions = %#o, want 0600", got)
	}
}

func TestArtifactStoreV3FailedPutLeavesNoFiles(t *testing.T) {
	store, err := NewArtifactStoreV3(1024)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if err := store.Put(
		"module-0001.zip",
		strings.NewReader("corrupted"),
		9,
		artifactDigestV3("different"),
	); err == nil {
		t.Fatal("expected digest mismatch")
	}

	entries, err := os.ReadDir(store.dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("failed Put left %d files behind", len(entries))
	}
}

func TestArtifactStoreV3RejectsPutAfterClose(t *testing.T) {
	store, err := NewArtifactStoreV3(1024)
	if err != nil {
		t.Fatal(err)
	}

	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("repeated Close: %v", err)
	}

	if err := store.Put(
		"module-0001.zip",
		strings.NewReader("data"),
		4,
		artifactDigestV3("data"),
	); err == nil {
		t.Fatal("Put succeeded after Close")
	}
}
