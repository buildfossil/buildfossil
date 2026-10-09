package capsule

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFile(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "failure.bfc")

	manifest := Manifest{
		SchemaVersion: SchemaVersion,
		Execution: Execution{
			Argv:       []string{"/bin/sh", "-c", "exit 17"},
			WorkingDir: ".",
			ExitCode:   17,
		},
		Platform: Platform{
			OS:           "linux",
			Architecture: "amd64",
		},
	}

	file := WorkspaceFile{
		Path: "fixture.txt",
		Data: []byte("BUILD_FOSSIL_TEST_FAILURE\n"),
		Mode: 0600,
	}

	if err := WriteFile(output, manifest, file); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	info, err := os.Stat(output)
	if err != nil {
		t.Fatalf("output file missing: %v", err)
	}

	if !info.Mode().IsRegular() {
		t.Fatal("output is not a regular file")
	}

	if got := info.Mode().Perm(); got != 0600 {
		t.Errorf("permissions = %o, want 0600", got)
	}

	if info.Size() == 0 {
		t.Error("capsule is empty")
	}

	original, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}

	if err := WriteFile(output, manifest, file); err == nil {
		t.Fatal("expected overwrite rejection")
	}

	after, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}

	if string(original) != string(after) {
		t.Error("existing capsule was modified")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 1 || entries[0].Name() != "failure.bfc" {
		t.Errorf("unexpected directory entries: %v", entries)
	}
}
