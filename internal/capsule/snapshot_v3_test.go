package capsule

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func makeVerifiedSnapshotFixture(t *testing.T) VerifiedCapsuleV3 {
	t.Helper()

	manifest, files, artifacts := makeWorkspaceV3Fixture(t)

	var archive bytes.Buffer
	if err := WriteWorkspaceV3(
		&archive,
		manifest,
		files,
		artifacts,
	); err != nil {
		t.Fatalf("write v3 fixture: %v", err)
	}

	filename := filepath.Join(t.TempDir(), "fixture.bfc")
	if err := os.WriteFile(filename, archive.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}

	verified, err := ReadVerifiedV3(filename)
	if err != nil {
		t.Fatalf("read verified fixture: %v", err)
	}

	return verified
}

func TestBuildWorkspaceSnapshotV3Contents(t *testing.T) {
	verified := makeVerifiedSnapshotFixture(t)

	snapshot, err := BuildWorkspaceSnapshotV3(verified)
	if err != nil {
		t.Fatalf("build snapshot: %v", err)
	}

	expected := make(map[string][]byte, len(verified.Files))
	for _, file := range verified.Files {
		expected[file.Path] = file.Data
	}

	reader := tar.NewReader(bytes.NewReader(snapshot))
	seen := make(map[string]bool)

	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read snapshot TAR: %v", err)
		}

		if header.Typeflag != tar.TypeReg {
			t.Fatalf("unexpected TAR type: %q", header.Name)
		}

		data, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("read %q: %v", header.Name, err)
		}

		want, exists := expected[header.Name]
		if !exists {
			t.Fatalf("unexpected file: %q", header.Name)
		}

		if seen[header.Name] {
			t.Fatalf("duplicate TAR entry: %q", header.Name)
		}
		seen[header.Name] = true

		if !bytes.Equal(data, want) {
			t.Fatalf("incorrect contents: %q", header.Name)
		}
	}

	if len(seen) != len(expected) {
		t.Fatalf("got %d files, want %d", len(seen), len(expected))
	}
}

func TestBuildWorkspaceSnapshotV3RejectsMutation(t *testing.T) {
	verified := makeVerifiedSnapshotFixture(t)

	verified.Files[0].Data = []byte("tampered content")

	if _, err := BuildWorkspaceSnapshotV3(verified); err == nil {
		t.Fatal("accepted modified workspace content")
	}
}

func TestBuildWorkspaceSnapshotV3RejectsDuplicate(t *testing.T) {
	verified := makeVerifiedSnapshotFixture(t)

	verified.Files[1] = verified.Files[0]

	if _, err := BuildWorkspaceSnapshotV3(verified); err == nil {
		t.Fatal("accepted duplicate workspace path")
	}
}

func TestBuildWorkspaceSnapshotV3RejectsTraversal(t *testing.T) {
	verified := makeVerifiedSnapshotFixture(t)

	verified.Files[0].Path = "../escape.txt"

	if _, err := BuildWorkspaceSnapshotV3(verified); err == nil {
		t.Fatal("accepted path traversal")
	}
}

func TestBuildWorkspaceSnapshotV3RejectsMissingFile(t *testing.T) {
	verified := makeVerifiedSnapshotFixture(t)

	verified.Files = verified.Files[:len(verified.Files)-1]

	if _, err := BuildWorkspaceSnapshotV3(verified); err == nil {
		t.Fatal("accepted missing workspace file")
	}
}

func TestBuildWorkspaceSnapshotV3Deterministic(t *testing.T) {
	verified := makeVerifiedSnapshotFixture(t)

	first, err := BuildWorkspaceSnapshotV3(verified)
	if err != nil {
		t.Fatal(err)
	}

	for i, j := 0, len(verified.Files)-1; i < j; i, j = i+1, j-1 {
		verified.Files[i], verified.Files[j] =
			verified.Files[j], verified.Files[i]
	}

	second, err := BuildWorkspaceSnapshotV3(verified)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(first, second) {
		t.Fatal("snapshot depends on input ordering")
	}
}

func TestBuildWorkspaceSnapshotV3RejectsWrongSchema(t *testing.T) {
	verified := makeVerifiedSnapshotFixture(t)

	verified.Manifest.SchemaVersion = SchemaVersionV2

	if _, err := BuildWorkspaceSnapshotV3(verified); err == nil {
		t.Fatal("accepted incorrect schema version")
	}
}

func TestBuildWorkspaceSnapshotV3RejectsSymlinkMode(t *testing.T) {
	verified := makeVerifiedSnapshotFixture(t)

	verified.Files[0].Mode = os.ModeSymlink | 0777

	if _, err := BuildWorkspaceSnapshotV3(verified); err == nil {
		t.Fatal("accepted symlink file mode")
	}
}

func TestBuildWorkspaceSnapshotV3RejectsOversizedFile(t *testing.T) {
	verified := makeVerifiedSnapshotFixture(t)

	verified.Files[0].Data = make([]byte, int(MaxWorkspaceFileSize)+1)

	if _, err := BuildWorkspaceSnapshotV3(verified); err == nil {
		t.Fatal("accepted oversized workspace file")
	}
}
