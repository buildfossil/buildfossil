package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/buildfossil/buildfossil/internal/capsule"
	"github.com/buildfossil/buildfossil/internal/replay"
)

func TestCaptureCommandV2Failure(t *testing.T) {
	workspace := t.TempDir()

	if err := os.Mkdir(
		filepath.Join(workspace, "src"),
		0700,
	); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(
		filepath.Join(workspace, "go.mod"),
		[]byte("module example.com/test\n"),
		0600,
	); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(
		filepath.Join(workspace, "src", "main.go"),
		[]byte("package main\n"),
		0600,
	); err != nil {
		t.Fatal(err)
	}

	output := filepath.Join(t.TempDir(), "failure.bfc")

	exitCode := captureCommandV2(
		[]string{"/bin/sh", "-c", "exit 23"},
		workspace,
		[]string{"go.mod", "src/main.go"},
		output,
	)

	if exitCode != 23 {
		t.Fatalf("exit code = %d, want 23", exitCode)
	}

	verified, err := capsule.ReadVerifiedV2(output)
	if err != nil {
		t.Fatalf("read captured capsule: %v", err)
	}

	if verified.Manifest.SchemaVersion != capsule.SchemaVersionV2 {
		t.Fatal("expected schema version 2")
	}

	if verified.Manifest.Execution.ExitCode != 23 {
		t.Fatal("incorrect original exit code")
	}

	if verified.Manifest.Platform.OS != runtime.GOOS ||
		verified.Manifest.Platform.Architecture != runtime.GOARCH {
		t.Fatal("incorrect captured platform")
	}

	if len(verified.Files) != 2 {
		t.Fatalf("file count = %d, want 2", len(verified.Files))
	}

	if verified.Files[0].Path != "go.mod" ||
		string(verified.Files[0].Data) != "module example.com/test\n" {
		t.Fatal("incorrect go.mod content")
	}

	if verified.Files[1].Path != "src/main.go" ||
		string(verified.Files[1].Data) != "package main\n" {
		t.Fatal("incorrect src/main.go content")
	}
}

func TestCaptureCommandV2SuccessDoesNotCreateArchive(t *testing.T) {
	workspace := t.TempDir()
	output := filepath.Join(t.TempDir(), "failure.bfc")

	if err := os.WriteFile(
		filepath.Join(workspace, "go.mod"),
		[]byte("module example.com/test\n"),
		0600,
	); err != nil {
		t.Fatal(err)
	}

	exitCode := captureCommandV2(
		[]string{"/bin/sh", "-c", "exit 0"},
		workspace,
		[]string{"go.mod"},
		output,
	)

	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0", exitCode)
	}

	_, err := os.Lstat(output)
	if !os.IsNotExist(err) {
		t.Fatalf(
			"expected no archive after successful command, stat error = %v",
			err,
		)
	}
}

func TestCaptureCommandV2DoesNotOverwriteArchive(t *testing.T) {
	workspace := t.TempDir()
	output := filepath.Join(t.TempDir(), "failure.bfc")

	if err := os.WriteFile(
		filepath.Join(workspace, "go.mod"),
		[]byte("module example.com/test\n"),
		0600,
	); err != nil {
		t.Fatal(err)
	}

	original := []byte("EXISTING_CAPSULE_DO_NOT_OVERWRITE\n")

	if err := os.WriteFile(output, original, 0600); err != nil {
		t.Fatal(err)
	}

	exitCode := captureCommandV2(
		[]string{"/bin/sh", "-c", "exit 23"},
		workspace,
		[]string{"go.mod"},
		output,
	)

	if exitCode != 125 {
		t.Fatalf("exit code = %d, want 125", exitCode)
	}

	content, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(content, original) {
		t.Fatal("existing capsule was modified")
	}
}

func TestCaptureReplayV2EndToEnd(t *testing.T) {
	if os.Getenv("BUILDFOSSIL_DOCKER_TEST") != "1" {
		t.Skip("set BUILDFOSSIL_DOCKER_TEST=1 to run Docker E2E test")
	}

	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("capture/replay E2E requires linux/amd64")
	}

	workspace := t.TempDir()

	if err := os.Mkdir(
		filepath.Join(workspace, "src"),
		0700,
	); err != nil {
		t.Fatal(err)
	}

	inputs := map[string][]byte{
		"go.mod":      []byte("module example.com/e2e\n"),
		"src/main.go": []byte("package main\n"),
	}

	for name, data := range inputs {
		destination := filepath.Join(workspace, filepath.FromSlash(name))

		if err := os.WriteFile(destination, data, 0600); err != nil {
			t.Fatal(err)
		}
	}

	output := filepath.Join(t.TempDir(), "failure.bfc")

	// Capture runs the original command on the test host.
	exitCode := captureCommandV2(
		[]string{
			"/bin/sh",
			"-c",
			"cat go.mod >&2; cat src/main.go >&2; exit 23",
		},
		workspace,
		[]string{"go.mod", "src/main.go"},
		output,
	)

	if exitCode != 23 {
		t.Fatalf("capture exit = %d, want 23", exitCode)
	}

	verified, err := capsule.ReadVerifiedV2(output)
	if err != nil {
		t.Fatalf("verify captured capsule: %v", err)
	}

	if len(verified.Files) != 2 {
		t.Fatalf("captured file count = %d, want 2", len(verified.Files))
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)
	defer cancel()

	result, err := replay.RunV2WithOptions(
		ctx,
		output,
		replay.Options{AllowArbitraryCommand: true},
	)
	if err != nil {
		t.Fatalf("replay v2: %v", err)
	}

	if result.OriginalExitCode != 23 ||
		result.ReplayExitCode != 23 ||
		result.Outcome != replay.OutcomeReproduced {
		t.Fatalf("unexpected E2E result: %+v", result)
	}
}
