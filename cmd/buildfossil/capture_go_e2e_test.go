package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/buildfossil/buildfossil/internal/capsule"
	"github.com/buildfossil/buildfossil/internal/replay"
)

func TestCaptureReplayGoBuildEndToEnd(t *testing.T) {
	if os.Getenv("BUILDFOSSIL_DOCKER_TEST") != "1" {
		t.Skip("Docker integration test disabled")
	}

	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("Go capture/replay E2E requires linux/amd64")
	}

	version, err := exec.Command("go", "version").Output()
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(
		string(version),
		"go"+capsule.SupportedGoVersion+" ",
	) {
		t.Fatalf("unsupported Go toolchain: %s", version)
	}

	if os.Geteuid() == 0 {
		t.Skip("Replay v2 requires non-root execution")
	}

	workspace := t.TempDir()

	files := map[string]string{
		"go.mod":  "module example.com/buildfossil-e2e\n\ngo 1.26.0\n",
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

	capsulePath := filepath.Join(t.TempDir(), "go-failure.bfc")

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
		t.Fatal(err)
	}

	if verified.Manifest.Runtime == nil {
		t.Fatal("Go runtime missing from captured manifest")
	}

	if verified.Manifest.Runtime.Kind != "go" ||
		verified.Manifest.Runtime.Version != capsule.SupportedGoVersion {
		t.Fatalf("unexpected runtime: %+v", verified.Manifest.Runtime)
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
		t.Fatalf("Go replay failed: %v", err)
	}

	if result.Outcome != replay.OutcomeReproduced {
		t.Fatalf(
			"outcome = %q, want %q (original exit=%d, replay exit=%d)",
			result.Outcome,
			replay.OutcomeReproduced,
			result.OriginalExitCode,
			result.ReplayExitCode,
		)
	}

	t.Logf("Capture → Replay outcome: %s", result.Outcome)
}
