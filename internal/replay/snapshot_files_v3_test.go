package replay

import (
	"bytes"
	"errors"
	"io"
	"path/filepath"

	"github.com/rewtools/rew/internal/capsule"
	"os"
	"testing"
)

func TestSnapshotFilesV3CloseNil(t *testing.T) {
	var files *snapshotFilesV3
	if err := files.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestSnapshotFilesV3CloseRemovesPrivateDirectory(t *testing.T) {
	directory, err := os.MkdirTemp("", "buildfossil-test-replay-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(directory) })

	info, err := os.Stat(directory)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0700 {
		t.Fatalf("directory permissions: %o", info.Mode().Perm())
	}

	files := &snapshotFilesV3{directory: directory}
	if err := files.Close(); err != nil {
		t.Fatal(err)
	}
	if err := files.Close(); err != nil {
		t.Fatalf("repeated Close: %v", err)
	}
	if _, err := os.Stat(directory); !os.IsNotExist(err) {
		t.Fatalf("temporary directory remains: %v", err)
	}
}

// verifiedSnapshotFilesFixtureV3 creates a real verified capsule
// without external Go dependencies or Docker.
func verifiedSnapshotFilesFixtureV3(
	t *testing.T,
) capsule.VerifiedCapsuleV3 {
	t.Helper()

	files := []capsule.WorkspaceFile{
		{
			Path: "go.mod",
			Data: []byte("module example.com/replay-snapshot-test\n\ngo 1.26.0\n"),
			Mode: 0600,
		},
		{
			Path: "go.sum",
			Data: []byte{},
			Mode: 0600,
		},
		{
			Path: "main.go",
			Data: []byte("package main\nfunc main() { missingSymbol() }\n"),
			Mode: 0600,
		},
	}

	manifest := capsule.Manifest{
		SchemaVersion: capsule.SchemaVersionV3,
		Execution: capsule.Execution{
			Argv:       []string{"go", "build", "./..."},
			WorkingDir: ".",
			ExitCode:   1,
			Stderr:     "undefined: missingSymbol",
		},
		Platform: capsule.Platform{
			OS:           "linux",
			Architecture: "amd64",
		},
		Runtime: &capsule.Runtime{
			Kind:    "go",
			Version: capsule.SupportedGoVersion,
		},
		GoBuildEnv: &capsule.GoBuildEnvironment{
			GOOS:       "linux",
			GOARCH:     "amd64",
			CGOEnabled: "0",
		},
	}

	var encoded bytes.Buffer
	if err := capsule.WriteWorkspaceV3(
		&encoded, manifest, files, map[string][]byte{},
	); err != nil {
		t.Fatalf("write fixture capsule: %v", err)
	}

	path := filepath.Join(t.TempDir(), "fixture.bfc")
	if err := os.WriteFile(path, encoded.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}

	verified, err := capsule.ReadVerifiedV3(path)
	if err != nil {
		t.Fatalf("verify fixture capsule: %v", err)
	}

	return verified
}

func TestSnapshotFilesV3PreparedArchives(t *testing.T) {
	verified := verifiedSnapshotFilesFixtureV3(t)

	prepared, err := prepareSnapshotFilesV3(verified)
	if err != nil {
		t.Fatalf("prepare TAR files: %v", err)
	}
	defer func() {
		if err := prepared.Close(); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	}()

	directory := prepared.directory

	dirInfo, err := os.Stat(directory)
	if err != nil {
		t.Fatal(err)
	}
	if dirInfo.Mode().Perm() != 0700 {
		t.Fatalf("directory permissions: %o", dirInfo.Mode().Perm())
	}

	for _, tc := range []struct {
		name  string
		file  *os.File
		build func(capsule.VerifiedCapsuleV3) ([]byte, error)
	}{
		{"workspace.tar", prepared.workspace, capsule.BuildWorkspaceSnapshotV3},
		{"modules.tar", prepared.modules, capsule.BuildGoModuleSnapshotV3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			info, err := tc.file.Stat()
			if err != nil {
				t.Fatal(err)
			}
			if !info.Mode().IsRegular() ||
				info.Mode().Perm() != 0600 {
				t.Fatalf("invalid TAR file mode: %v", info.Mode())
			}

			expected, err := tc.build(verified)
			if err != nil {
				t.Fatal(err)
			}
			actual, err := io.ReadAll(tc.file)
			if err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal(actual, expected) {
				t.Fatalf("prepared %s differs from snapshot API", tc.name)
			}

			// Rewind for the future Docker reader consumer.
			if _, err := tc.file.Seek(0, io.SeekStart); err != nil {
				t.Fatal(err)
			}
		})
	}

	if err := prepared.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(directory); !os.IsNotExist(err) {
		t.Fatalf("temporary directory remains: %v", err)
	}
}

func TestSnapshotFilesV3FailedSecondArchiveCleansUp(t *testing.T) {
	// Isolate temporary files so a failed preparation can be checked
	// without inspecting unrelated process/system temp directories.
	temporaryRoot := t.TempDir()
	t.Setenv("TMPDIR", temporaryRoot)

	verified := verifiedSnapshotFilesFixtureV3(t)

	// Workspace TAR remains valid, but an unexpected module artifact
	// makes module TAR verification fail after workspace is written.
	verified.Artifacts = map[string][]byte{
		"module-0001.zip": []byte("unexpected"),
	}

	prepared, err := prepareSnapshotFilesV3(verified)
	if err == nil {
		if prepared != nil {
			_ = prepared.Close()
		}
		t.Fatal("invalid module artifact accepted")
	}
	if prepared != nil {
		_ = prepared.Close()
		t.Fatal("failed preparation returned snapshot readers")
	}

	remaining, err := os.ReadDir(temporaryRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 0 {
		t.Fatalf("temporary files leaked after failure: %v", remaining)
	}
}

func TestSnapshotFilesV3CleanupError(t *testing.T) {
	directory := t.TempDir()
	nested, err := os.MkdirTemp(directory, "snapshot-*")
	if err != nil {
		t.Fatal(err)
	}

	file, err := os.CreateTemp(nested, "workspace-*")
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	files := &snapshotFilesV3{
		directory: nested,
		workspace: file, // Already closed: Close must report failure.
	}

	primary := errors.New("original replay failure")
	combined := closeSnapshotFilesV3(files, primary)

	if !errors.Is(combined, primary) {
		t.Fatalf("original error lost: %v", combined)
	}
	if combined == primary {
		t.Fatal("cleanup error was ignored")
	}
	if _, err := os.Stat(nested); !os.IsNotExist(err) {
		t.Fatalf("snapshot directory remains: %v", err)
	}
}

func TestSnapshotFilesV3CleanupPreservesSuccess(t *testing.T) {
	files := &snapshotFilesV3{}
	if err := closeSnapshotFilesV3(files, nil); err != nil {
		t.Fatalf("unexpected cleanup error: %v", err)
	}
}
