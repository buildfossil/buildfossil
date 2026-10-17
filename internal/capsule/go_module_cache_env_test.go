package capsule

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInspectGoModuleCacheWithExecutable(t *testing.T) {
	dir := t.TempDir()
	goExecutable := filepath.Join(dir, "fake-go")
	cacheDir := filepath.Join(dir, "module-cache")

	script := "#!/bin/sh\n" +
		"if [ \"$GOTOOLCHAIN\" != local ]; then exit 9; fi\n" +
		"printf '%s\\n' '" + cacheDir + "'\n"

	if err := os.WriteFile(goExecutable, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}

	t.Setenv("GOTOOLCHAIN", "auto")

	got, err := InspectGoModuleCacheWithExecutable(
		dir, goExecutable,
	)
	if err != nil {
		t.Fatal(err)
	}
	if got != cacheDir {
		t.Fatalf("GOMODCACHE = %q, want %q", got, cacheDir)
	}
}

func TestInspectGoModuleCacheRejectsRelativeExecutable(t *testing.T) {
	_, err := InspectGoModuleCacheWithExecutable(
		t.TempDir(), "go",
	)
	if err == nil {
		t.Fatal("expected relative executable rejection")
	}
}

func TestInspectGoModuleCacheRejectsEmptyOutput(t *testing.T) {
	dir := t.TempDir()
	goExecutable := filepath.Join(dir, "fake-go")

	if err := os.WriteFile(goExecutable, []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}

	_, err := InspectGoModuleCacheWithExecutable(dir, goExecutable)
	if err == nil {
		t.Fatal("expected empty GOMODCACHE error")
	}
}
