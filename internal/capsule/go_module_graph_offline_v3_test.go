package capsule

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestListGoModuleGraphV3RejectsMissingOfflineDependency(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("Go toolchain unavailable")
	}

	workspace := t.TempDir()
	cache := t.TempDir()

	goMod := `module example.com/buildfossil-offline-test

go 1.22

require github.com/spf13/pflag v1.0.9
`

	if err := os.WriteFile(
		filepath.Join(workspace, "go.mod"),
		[]byte(goMod),
		0600,
	); err != nil {
		t.Fatal(err)
	}

	_, err := ListGoModuleGraphV3(workspace, cache)
	if err == nil {
		t.Fatal("missing offline dependency unexpectedly accepted")
	}

	if !strings.Contains(err.Error(), "offline Go module graph failed") {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(cache, "cache", "download")); !os.IsNotExist(err) {
		t.Fatalf("unexpected module downloads in empty cache: %v", err)
	}
}
