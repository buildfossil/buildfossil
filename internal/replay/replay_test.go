package replay

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buildfossil/buildfossil/internal/capsule"
)

func TestReplayIntegration(t *testing.T) {
	if os.Getenv("BUILDFOSSIL_DOCKER_TEST") != "1" {
		t.Skip("Docker integration test disabled")
	}

	dir := t.TempDir()
	output := filepath.Join(dir, "failure.bfc")

	manifest := capsule.Manifest{
		SchemaVersion: capsule.SchemaVersion,
		Execution: capsule.Execution{
			Argv: []string{
				"/bin/sh",
				"-c",
				"cat fixture.txt >&2; exit 17",
			},
			WorkingDir: ".",
			ExitCode:   17,
			Stderr:     "BUILD_FOSSIL_TEST_FAILURE\n",
		},
		Platform: capsule.Platform{
			OS:           "linux",
			Architecture: "amd64",
		},
	}

	file := capsule.WorkspaceFile{
		Path: "fixture.txt",
		Data: []byte("BUILD_FOSSIL_TEST_FAILURE\n"),
		Mode: 0600,
	}

	if err := capsule.WriteFile(output, manifest, file); err != nil {
		t.Fatalf("create capsule: %v", err)
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)
	defer cancel()

	result, err := Run(ctx, output)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}

	if result.OriginalExitCode != 17 {
		t.Errorf(
			"original exit = %d, want 17",
			result.OriginalExitCode,
		)
	}

	if result.ReplayExitCode != 17 {
		t.Errorf(
			"replay exit = %d, want 17",
			result.ReplayExitCode,
		)
	}

	if result.Outcome != OutcomeReproduced {
		t.Fatalf(
			"replay outcome = %q, want %q",
			result.Outcome,
			OutcomeReproduced,
		)
	}
}

func TestReplayRejectsUnsupportedPlatform(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "macos.bfc")

	manifest := capsule.Manifest{
		SchemaVersion: capsule.SchemaVersion,
		Execution: capsule.Execution{
			Argv: []string{
				"/bin/sh",
				"-c",
				"cat fixture.txt >&2; exit 17",
			},
			WorkingDir: ".",
			ExitCode:   17,
			Stderr:     "BUILD_FOSSIL_TEST_FAILURE\n",
		},
		Platform: capsule.Platform{
			OS:           "darwin",
			Architecture: "arm64",
		},
	}

	file := capsule.WorkspaceFile{
		Path: "fixture.txt",
		Data: []byte("BUILD_FOSSIL_TEST_FAILURE\n"),
		Mode: 0600,
	}

	if err := capsule.WriteFile(output, manifest, file); err != nil {
		t.Fatalf("create capsule: %v", err)
	}

	_, err := Run(context.Background(), output)
	if err == nil {
		t.Fatal("expected unsupported platform error")
	}

	if !strings.Contains(err.Error(), "unsupported source platform") {
		t.Fatalf("unexpected error: %v", err)
	}
}
