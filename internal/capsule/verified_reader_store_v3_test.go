package capsule

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func storedReaderFixtureV3(t *testing.T) string {
	t.Helper()

	verified := makeVerifiedSnapshotFixture(t)

	var archive bytes.Buffer
	if err := WriteWorkspaceV3(
		&archive,
		verified.Manifest,
		verified.Files,
		verified.Artifacts,
	); err != nil {
		t.Fatal(err)
	}

	filename := filepath.Join(t.TempDir(), "fixture.bfc")
	if err := os.WriteFile(filename, archive.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	return filename
}

func TestReadVerifiedToStoreV3MatchesLegacy(t *testing.T) {
	filename := storedReaderFixtureV3(t)

	legacy, err := ReadVerifiedV3(filename)
	if err != nil {
		t.Fatalf("legacy verification: %v", err)
	}

	stored, err := ReadVerifiedToStoreV3(filename)
	if err != nil {
		t.Fatalf("disk verification: %v", err)
	}
	directory := stored.Store.dir

	if !reflect.DeepEqual(legacy.Manifest, stored.Manifest) {
		t.Fatal("manifests differ")
	}
	if !reflect.DeepEqual(legacy.Files, stored.Files) {
		t.Fatal("workspace files differ")
	}

	if len(legacy.Artifacts) != len(stored.Store.names) {
		t.Fatal("artifact counts differ")
	}

	for name, expected := range legacy.Artifacts {
		file, err := stored.Store.Open(name)
		if err != nil {
			t.Fatal(err)
		}
		actual, readErr := io.ReadAll(file)
		closeErr := file.Close()
		if readErr != nil || closeErr != nil {
			t.Fatalf("read %s: %v %v", name, readErr, closeErr)
		}
		if !bytes.Equal(actual, expected) {
			t.Fatalf("artifact content mismatch: %s", name)
		}
	}

	if err := stored.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(directory); !os.IsNotExist(err) {
		t.Fatalf("store directory leaked: %v", err)
	}
}

func TestReadVerifiedToStoreV3RejectsCorruptArtifact(t *testing.T) {
	filename := storedReaderFixtureV3(t)

	original, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}

	legacy, err := ReadVerifiedV3(filename)
	if err != nil {
		t.Fatal(err)
	}

	var artifact []byte
	for _, mod := range legacy.Manifest.GoModules {
		for _, metadata := range mod.Artifacts {
			if len(legacy.Artifacts[metadata.Path]) > 20 {
				artifact = legacy.Artifacts[metadata.Path]
				break
			}
		}
	}
	if len(artifact) == 0 {
		t.Skip("fixture lacks a suitable nonempty artifact")
	}

	offset := bytes.Index(original, artifact)
	if offset < 0 {
		t.Fatal("artifact bytes not found in TAR")
	}

	corrupted := append([]byte(nil), original...)
	corrupted[offset+10] ^= 0x01

	bad := filepath.Join(t.TempDir(), "corrupt.bfc")
	if err := os.WriteFile(bad, corrupted, 0600); err != nil {
		t.Fatal(err)
	}

	if _, err := ReadVerifiedV3(bad); err == nil {
		t.Fatal("legacy reader accepted corrupted artifact")
	}
	stored, err := ReadVerifiedToStoreV3(bad)
	if err == nil {
		_ = stored.Close()
		t.Fatal("disk reader accepted corrupted artifact")
	}
}

func TestReadVerifiedToStoreV3RejectsSymlink(t *testing.T) {
	filename := storedReaderFixtureV3(t)
	link := filepath.Join(t.TempDir(), "linked.bfc")
	if err := os.Symlink(filename, link); err != nil {
		t.Fatal(err)
	}

	stored, err := ReadVerifiedToStoreV3(link)
	if err == nil {
		_ = stored.Close()
		t.Fatal("symlink capsule accepted")
	}
}

func TestReadVerifiedToStoreV3CleansUpOnFailure(t *testing.T) {
	filename := storedReaderFixtureV3(t)

	encoded, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}

	legacy, err := ReadVerifiedV3(filename)
	if err != nil {
		t.Fatal(err)
	}

	var artifact []byte
	for _, mod := range legacy.Manifest.GoModules {
		for _, metadata := range mod.Artifacts {
			candidate := legacy.Artifacts[metadata.Path]
			if len(candidate) > 20 {
				artifact = candidate
				break
			}
		}
		if len(artifact) != 0 {
			break
		}
	}

	if len(artifact) == 0 {
		t.Fatal("fixture has no suitable artifact")
	}

	offset := bytes.Index(encoded, artifact)
	if offset < 0 {
		t.Fatal("artifact not found in TAR")
	}

	corrupted := append([]byte(nil), encoded...)
	corrupted[offset+10] ^= 1

	bad := filepath.Join(t.TempDir(), "corrupt.bfc")
	if err := os.WriteFile(bad, corrupted, 0600); err != nil {
		t.Fatal(err)
	}

	tempRoot := t.TempDir()
	t.Setenv("TMPDIR", tempRoot)

	stored, err := ReadVerifiedToStoreV3(bad)
	if err == nil {
		_ = stored.Close()
		t.Fatal("corrupted artifact accepted")
	}

	remaining, err := os.ReadDir(tempRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 0 {
		t.Fatalf(
			"artifact store leaked after failure: %v",
			remaining,
		)
	}
}
