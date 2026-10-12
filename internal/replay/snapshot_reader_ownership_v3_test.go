package replay

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestSnapshotReaderOwnershipV3(t *testing.T) {
	directory := t.TempDir()
	ownedDirectory, err := os.MkdirTemp(
		directory, "snapshots-*",
	)
	if err != nil {
		t.Fatal(err)
	}

	workspaceData := []byte("workspace archive")
	moduleData := []byte("module archive")

	openReader := func(name string, data []byte) *os.File {
		t.Helper()

		filename := filepath.Join(ownedDirectory, name)
		if err := os.WriteFile(filename, data, 0600); err != nil {
			t.Fatal(err)
		}

		file, err := os.Open(filename)
		if err != nil {
			t.Fatal(err)
		}
		return file
	}

	snapshots := &snapshotFilesV3{
		directory: ownedDirectory,
		workspace: openReader("workspace.tar", workspaceData),
		modules:   openReader("modules.tar", moduleData),
	}

	t.Cleanup(func() {
		_ = snapshots.Close()
	})

	for _, tc := range []struct {
		name     string
		file     *os.File
		expected []byte
	}{
		{"workspace", snapshots.workspace, workspaceData},
		{"modules", snapshots.modules, moduleData},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Simulate Docker SDK closing the supplied wrapper.
			borrowed := io.NopCloser(tc.file)

			received, err := io.ReadAll(borrowed)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(received, tc.expected) {
				t.Fatal("borrowed reader content mismatch")
			}

			if err := borrowed.Close(); err != nil {
				t.Fatal(err)
			}

			// The actual file must remain owned by Replay.
			if _, err := tc.file.Seek(0, io.SeekStart); err != nil {
				t.Fatalf("borrowed Close closed owner: %v", err)
			}

			again, err := io.ReadAll(tc.file)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(again, tc.expected) {
				t.Fatal("owner file content mismatch")
			}
		})
	}

	workspace := snapshots.workspace
	modules := snapshots.modules

	if err := snapshots.Close(); err != nil {
		t.Fatalf("owner cleanup failed: %v", err)
	}

	for _, file := range []*os.File{workspace, modules} {
		if _, err := file.Stat(); !os.IsNotExist(err) &&
			!isClosedFileErrorV3(err) {
			t.Fatalf("owner failed to close file: %v", err)
		}
	}

	if _, err := os.Stat(ownedDirectory); !os.IsNotExist(err) {
		t.Fatalf("snapshot directory remains: %v", err)
	}

	if err := snapshots.Close(); err != nil {
		t.Fatalf("repeated owner cleanup failed: %v", err)
	}
}

func isClosedFileErrorV3(err error) bool {
	return os.IsNotExist(err) || err == os.ErrClosed ||
		isClosedFileWrappedV3(err)
}

func isClosedFileWrappedV3(err error) bool {
	return errors.Is(err, os.ErrClosed)
}
