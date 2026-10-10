package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/buildfossil/buildfossil/internal/capsule"
)

func TestCaptureV2DetectsGoRuntime(t *testing.T) {
	output, err := exec.Command("go", "version").Output()
	if err != nil {
		t.Skipf("Go toolchain unavailable: %v", err)
	}

	if !strings.Contains(
		string(output),
		"go"+capsule.SupportedGoVersion+" ",
	) {
		t.Skipf("unsupported Go version: %s", output)
	}

	workspace := t.TempDir()

	files := map[string]string{
		"go.mod":  "module example.com/capture-probe\n\ngo 1.26.0\n",
		"main.go": "package main\n\nfunc main() {\n    undefinedFunction()\n}\n",
	}

	for name, content := range files {
		if err := os.WriteFile(
			filepath.Join(workspace, name),
			[]byte(content),
			0600,
		); err != nil {
			t.Fatal(err)
		}
	}

	capsulePath := filepath.Join(t.TempDir(), "failure.bfc")

	exitCode := captureCommandV2(
		[]string{"go", "build", "./..."},
		workspace,
		[]string{"go.mod", "main.go"},
		capsulePath,
	)

	if exitCode != 1 {
		t.Fatalf("capture exit code = %d, want 1", exitCode)
	}

	verified, err := capsule.ReadVerifiedV2(capsulePath)
	if err != nil {
		t.Fatalf("read captured capsule: %v", err)
	}

	if verified.Manifest.Runtime == nil {
		t.Fatal("captured manifest has no runtime")
	}

	if got := verified.Manifest.Runtime.Kind; got != "go" {
		t.Fatalf("runtime kind = %q, want go", got)
	}

	if got := verified.Manifest.Runtime.Version; got != capsule.SupportedGoVersion {
		t.Fatalf(
			"runtime version = %q, want %q",
			got,
			capsule.SupportedGoVersion,
		)
	}

	if got := verified.Manifest.Execution.ExitCode; got != 1 {
		t.Fatalf("recorded exit code = %d, want 1", got)
	}
}
