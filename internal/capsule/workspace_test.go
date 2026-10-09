package capsule

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadWorkspaceFile(t *testing.T) {
	root := t.TempDir()
	content := []byte("BUILD_FOSSIL_TEST_FAILURE\n")

	file := filepath.Join(root, "fixture.txt")

	if err := os.WriteFile(file, content, 0600); err != nil {
		t.Fatal(err)
	}

	got, err := ReadWorkspaceFile(root, "fixture.txt")
	if err != nil {
		t.Fatalf("ReadWorkspaceFile() error = %v", err)
	}

	if got.Path != "fixture.txt" {
		t.Errorf("path = %q, want fixture.txt", got.Path)
	}

	if string(got.Data) != string(content) {
		t.Errorf("data = %q, want %q", got.Data, content)
	}

	if got.Mode.Perm() != 0600 {
		t.Errorf("mode = %o, want 0600", got.Mode.Perm())
	}
}

func TestReadWorkspaceFileRejectsUnsupportedPath(t *testing.T) {
	root := t.TempDir()

	_, err := ReadWorkspaceFile(root, "../secret.txt")
	if err == nil {
		t.Fatal("expected traversal rejection")
	}

	_, err = ReadWorkspaceFile(root, "other.txt")
	if err == nil {
		t.Fatal("expected unsupported path rejection")
	}
}

func TestReadWorkspaceFileRejectsSymlink(t *testing.T) {
	root := t.TempDir()

	target := filepath.Join(root, "target.txt")
	if err := os.WriteFile(target, []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}

	link := filepath.Join(root, "fixture.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	_, err := ReadWorkspaceFile(root, "fixture.txt")
	if err == nil {
		t.Fatal("expected symlink rejection")
	}
}
