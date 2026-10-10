package capsule

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestZeroDependencyV3ModuleSnapshot(t *testing.T) {
	data := zeroDependencyFixtureV3(t)

	filename := filepath.Join(t.TempDir(), "zero.bfc")
	if err := os.WriteFile(filename, data, 0600); err != nil {
		t.Fatal(err)
	}

	verified, err := ReadVerifiedV3(filename)
	if err != nil {
		t.Fatal(err)
	}

	snapshot, err := BuildGoModuleSnapshotV3(verified)
	if err != nil {
		t.Fatalf("build empty module snapshot: %v", err)
	}

	reader := tar.NewReader(bytes.NewReader(snapshot))

	if _, err := reader.Next(); err != io.EOF {
		t.Fatalf("expected empty TAR, got %v", err)
	}

	t.Run("reject unexpected artifacts", func(t *testing.T) {
		modified := verified
		modified.Artifacts = map[string][]byte{
			"module-0001.zip": []byte("unexpected"),
		}

		if _, err := BuildGoModuleSnapshotV3(modified); err == nil {
			t.Fatal("accepted undeclared module artifacts")
		}
	})

	t.Run("reject modified workspace", func(t *testing.T) {
		modified := verified
		modified.Files = append([]WorkspaceFile(nil), verified.Files...)
		modified.Files[0].Data = []byte("tampered go.mod")

		if _, err := BuildGoModuleSnapshotV3(modified); err == nil {
			t.Fatal("accepted modified workspace contents")
		}
	})
}
