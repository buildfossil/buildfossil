package capsule

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceV3FileDirectoryConflictsBothOrders(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)

	for _, paths := range [][]string{
		{"src", "src/main.go"},
		{"src/main.go", "src"},
	} {
		t.Run(strings.Join(paths, "_then_"), func(t *testing.T) {
			metadata := make([]FileMetadata, 0, len(paths))
			for _, name := range paths {
				metadata = append(metadata, FileMetadata{
					Path: name, Size: 1, SHA256: digest,
				})
			}
			if err := validateWorkspaceV3(Workspace{
				Files: metadata,
			}); err == nil {
				t.Fatal("expected file/directory conflict")
			}
		})
	}
}

func TestWorkspaceV3ReaderRejectsSymlinkAndTraversal(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}

	if err := os.Symlink(outside, filepath.Join(root, "link.txt")); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"link.txt", "../secret.txt"} {
		t.Run(name, func(t *testing.T) {
			if _, err := ReadWorkspaceFilesV3(root, []string{name}); err == nil {
				t.Fatalf("expected rejection of %q", name)
			}
		})
	}
}
