package capsule

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveGoExecutable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "go")

	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", dir)

	got, err := ResolveGoExecutable()
	if err != nil {
		t.Fatal(err)
	}

	if got != path {
		t.Fatalf("resolved path = %q, want %q", got, path)
	}
}

func TestResolveGoExecutableMissing(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	_, err := ResolveGoExecutable()
	if err == nil || !strings.Contains(err.Error(), "locate Go executable") {
		t.Fatalf("expected missing executable error, got %v", err)
	}
}
