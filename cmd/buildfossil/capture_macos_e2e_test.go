package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/buildfossil/buildfossil/internal/capsule"
	"github.com/buildfossil/buildfossil/internal/replay"
)

func TestCaptureReplayMacOSGoEndToEnd(t *testing.T) {
	if os.Getenv("BUILDFOSSIL_DOCKER_TEST") != "1" {
		t.Skip("Docker integration test disabled")
	}

	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		t.Skip("requires macOS ARM64")
	}

	if os.Geteuid() == 0 {
		t.Skip("Replay v2 requires non-root execution")
	}

	t.Setenv("CGO_ENABLED", "0")
	t.Setenv("GOFLAGS", "")
	t.Setenv("GOOS", "darwin")
	t.Setenv("GOARCH", "arm64")

	workspace := t.TempDir()

	files := map[string]string{
		"go.mod":  "module example.com/macos-probe\n\ngo 1.26.0\n",
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

	capsulePath := filepath.Join(t.TempDir(), "macos-failure.bfc")

	captureExit := captureCommandV2(
		[]string{"go", "build", "./..."},
		workspace,
		[]string{"go.mod", "main.go"},
		capsulePath,
	)

	if captureExit != 1 {
		t.Fatalf("capture exit = %d, want 1", captureExit)
	}

	verified, err := capsule.ReadVerifiedV2(capsulePath)
	if err != nil {
		t.Fatalf("verify capsule: %v", err)
	}

	if verified.Manifest.Runtime == nil ||
		verified.Manifest.GoBuildEnv == nil {
		t.Fatal("missing Go runtime or build environment")
	}

	if verified.Manifest.GoBuildEnv.CGOEnabled != "0" {
		t.Fatal("capture did not record CGO_ENABLED=0")
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		120*time.Second,
	)
	defer cancel()

	result, err := replay.RunV2WithOptions(
		ctx,
		capsulePath,
		replay.Options{AllowArbitraryCommand: true},
	)
	if err != nil {
		t.Fatalf("macOS to Docker replay: %v", err)
	}

	if result.Outcome != replay.OutcomeReproduced {
		t.Fatalf(
			"outcome = %q, original exit=%d, replay exit=%d",
			result.Outcome,
			result.OriginalExitCode,
			result.ReplayExitCode,
		)
	}

	t.Logf("macOS → Linux Docker outcome: %s", result.Outcome)
}
