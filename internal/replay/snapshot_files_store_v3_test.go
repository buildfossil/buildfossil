package replay

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/rewtools/rew/internal/capsule"
)

func storedSnapshotFixtureV3(
	t *testing.T,
) (
	capsule.VerifiedCapsuleV3,
	capsule.VerifiedStoredCapsuleV3,
) {
	t.Helper()

	legacy := verifiedSnapshotFilesFixtureV3(t)

	var encoded bytes.Buffer
	if err := capsule.WriteWorkspaceV3(
		&encoded,
		legacy.Manifest,
		legacy.Files,
		legacy.Artifacts,
	); err != nil {
		t.Fatal(err)
	}

	filename := filepath.Join(t.TempDir(), "stored.bfc")
	if err := os.WriteFile(
		filename, encoded.Bytes(), 0600,
	); err != nil {
		t.Fatal(err)
	}

	stored, err := capsule.ReadVerifiedToStoreV3(filename)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if err := stored.Close(); err != nil {
			t.Errorf("close stored fixture: %v", err)
		}
	})

	return legacy, stored
}

func TestSnapshotFilesFromStoreV3MatchesLegacy(t *testing.T) {
	legacy, stored := storedSnapshotFixtureV3(t)

	prepared, err := prepareSnapshotFilesFromStoreV3(stored)
	if err != nil {
		t.Fatal(err)
	}
	defer prepared.Close()

	oldWorkspace, err := capsule.BuildWorkspaceSnapshotV3(
		legacy,
	)
	if err != nil {
		t.Fatal(err)
	}

	oldModules, err := capsule.BuildGoModuleSnapshotV3(
		legacy,
	)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name string
		file *os.File
		want []byte
	}{
		{"workspace.tar", prepared.workspace, oldWorkspace},
		{"modules.tar", prepared.modules, oldModules},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stat, err := tc.file.Stat()
			if err != nil {
				t.Fatal(err)
			}
			if stat.Mode().Perm() != 0600 {
				t.Fatalf("unexpected TAR permissions: %v", stat.Mode())
			}

			actual, err := io.ReadAll(tc.file)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(actual, tc.want) {
				t.Fatalf("%s content mismatch", tc.name)
			}
		})
	}

	directory := prepared.directory
	if err := prepared.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(directory); !os.IsNotExist(err) {
		t.Fatalf("snapshot directory remains: %v", err)
	}
}

func TestSnapshotFilesFromStoreV3FailureCleansUp(t *testing.T) {
	_, stored := storedSnapshotFixtureV3(t)

	// Workspace TAR should succeed. Module TAR must fail because
	// the required artifact store is unavailable.
	stored.Store = nil

	tempRoot := t.TempDir()
	t.Setenv("TMPDIR", tempRoot)

	prepared, err := prepareSnapshotFilesFromStoreV3(stored)
	if err == nil {
		if prepared != nil {
			_ = prepared.Close()
		}
		t.Fatal("missing artifact store accepted")
	}

	if prepared != nil {
		_ = prepared.Close()
		t.Fatal("failed preparation returned readers")
	}

	entries, err := os.ReadDir(tempRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("snapshot files leaked: %v", entries)
	}
}
