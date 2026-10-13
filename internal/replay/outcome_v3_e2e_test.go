package replay

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rewtools/rew/internal/capsule"
)

func TestReplayV3AmbiguousExit137E2E(t *testing.T) {
	if os.Getenv("REW_DOCKER_TEST") != "1" {
		t.Skip("Docker E2E disabled")
	}
	if os.Geteuid() == 0 {
		t.Skip("Docker runner refuses root")
	}

	ctx, cancel := context.WithTimeout(
		context.Background(), 180*time.Second,
	)
	defer cancel()

	files := []capsule.WorkspaceFile{
		{
			Path: "go.mod",
			Data: []byte("module example.com/replay-exit137\n\ngo 1.26.0\n"),
			Mode: 0600,
		},
		{
			Path: "go.sum",
			Data: []byte{},
			Mode: 0600,
		},
		{
			Path: "main.go",
			Data: []byte("package main\nfunc main() {}\n"),
			Mode: 0600,
		},
	}

	manifest := capsule.Manifest{
		SchemaVersion: capsule.SchemaVersionV3,
		Execution: capsule.Execution{
			Argv: []string{
				"/bin/sh", "-c",
				"printf 'expected-failure\\n' >&2; exit 137",
			},
			WorkingDir: ".",
			ExitCode:   137,
			Stderr:     "expected-failure\n",
		},
		Platform: capsule.Platform{
			OS:           "linux",
			Architecture: "amd64",
		},
		Runtime: &capsule.Runtime{
			Kind:    "go",
			Version: capsule.SupportedGoVersion,
		},
		GoBuildEnv: &capsule.GoBuildEnvironment{
			GOOS:       "linux",
			GOARCH:     "amd64",
			CGOEnabled: "0",
		},
	}

	var encoded bytes.Buffer
	if err := capsule.WriteWorkspaceV3(
		&encoded, manifest, files, map[string][]byte{},
	); err != nil {
		t.Fatalf("write capsule: %v", err)
	}

	capsulePath := filepath.Join(t.TempDir(), "exit137.bfc")
	if err := os.WriteFile(capsulePath, encoded.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}

	result, err := RunV3WithOptions(
		ctx,
		capsulePath,
		Options{AllowArbitraryCommand: true},
	)
	if err != nil {
		t.Fatalf("Replay v3: %v", err)
	}

	if result.OriginalExitCode != 137 || result.ReplayExitCode != 137 {
		t.Fatalf(
			"unexpected exit codes: original=%d replay=%d",
			result.OriginalExitCode,
			result.ReplayExitCode,
		)
	}
	if result.Outcome != OutcomeInconclusive {
		t.Fatalf(
			"outcome=%q, want=%q",
			result.Outcome,
			OutcomeInconclusive,
		)
	}
	if result.Difference != "" {
		t.Fatalf("unexpected difference=%q", result.Difference)
	}

	t.Logf("Replay v3 exit 137: %s", result.Outcome)
	cliBinary := filepath.Join(t.TempDir(), "buildfossil")
	build := exec.CommandContext(
		ctx, "go", "build", "-o", cliBinary, "../../cmd/rew",
	)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\\n%s", err, output)
	}

	cli := exec.CommandContext(
		ctx, cliBinary, "replay", "--v3", "--allow-command", capsulePath,
	)
	cliOutput, cliErr := cli.CombinedOutput()

	var exitErr *exec.ExitError
	if !errors.As(cliErr, &exitErr) {
		t.Fatalf("expected CLI exit 12, got err=%v; output=%s", cliErr, cliOutput)
	}
	if exitErr.ExitCode() != 12 {
		t.Fatalf("CLI exit=%d, want 12; output=%s", exitErr.ExitCode(), cliOutput)
	}
	if !strings.Contains(string(cliOutput), "Outcome:            inconclusive") {
		t.Fatalf("CLI missing inconclusive outcome:\\n%s", cliOutput)
	}

	t.Logf("CLI replay v3 exit 137: exit=%d", exitErr.ExitCode())
}

func TestReplayV3AmbiguousExit143E2E(t *testing.T) {
	if os.Getenv("REW_DOCKER_TEST") != "1" {
		t.Skip("Docker E2E disabled")
	}
	if os.Geteuid() == 0 {
		t.Skip("Docker runner refuses root")
	}

	ctx, cancel := context.WithTimeout(
		context.Background(), 180*time.Second,
	)
	defer cancel()

	files := []capsule.WorkspaceFile{
		{
			Path: "go.mod",
			Data: []byte("module example.com/replay-exit143\n\ngo 1.26.0\n"),
			Mode: 0600,
		},
		{
			Path: "go.sum",
			Data: []byte{},
			Mode: 0600,
		},
		{
			Path: "main.go",
			Data: []byte("package main\nfunc main() {}\n"),
			Mode: 0600,
		},
	}

	manifest := capsule.Manifest{
		SchemaVersion: capsule.SchemaVersionV3,
		Execution: capsule.Execution{
			Argv: []string{
				"/bin/sh", "-c",
				"printf 'expected-failure\\n' >&2; exit 143",
			},
			WorkingDir: ".",
			ExitCode:   143,
			Stderr:     "expected-failure\n",
		},
		Platform: capsule.Platform{
			OS:           "linux",
			Architecture: "amd64",
		},
		Runtime: &capsule.Runtime{
			Kind:    "go",
			Version: capsule.SupportedGoVersion,
		},
		GoBuildEnv: &capsule.GoBuildEnvironment{
			GOOS:       "linux",
			GOARCH:     "amd64",
			CGOEnabled: "0",
		},
	}

	var encoded bytes.Buffer
	if err := capsule.WriteWorkspaceV3(
		&encoded, manifest, files, map[string][]byte{},
	); err != nil {
		t.Fatalf("write capsule: %v", err)
	}

	capsulePath := filepath.Join(t.TempDir(), "exit143.bfc")
	if err := os.WriteFile(capsulePath, encoded.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}

	result, err := RunV3WithOptions(
		ctx,
		capsulePath,
		Options{AllowArbitraryCommand: true},
	)
	if err != nil {
		t.Fatalf("Replay v3: %v", err)
	}

	if result.OriginalExitCode != 143 || result.ReplayExitCode != 143 {
		t.Fatalf(
			"unexpected exit codes: original=%d replay=%d",
			result.OriginalExitCode,
			result.ReplayExitCode,
		)
	}
	if result.Outcome != OutcomeInconclusive {
		t.Fatalf(
			"outcome=%q, want=%q",
			result.Outcome,
			OutcomeInconclusive,
		)
	}
	if result.Difference != "" {
		t.Fatalf("unexpected difference=%q", result.Difference)
	}

	t.Logf("Replay v3 exit 143: %s", result.Outcome)
	cliBinary := filepath.Join(t.TempDir(), "buildfossil")
	build := exec.CommandContext(
		ctx, "go", "build", "-o", cliBinary, "../../cmd/rew",
	)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\\n%s", err, output)
	}

	cli := exec.CommandContext(
		ctx, cliBinary, "replay", "--v3", "--allow-command", capsulePath,
	)
	cliOutput, cliErr := cli.CombinedOutput()

	var exitErr *exec.ExitError
	if !errors.As(cliErr, &exitErr) {
		t.Fatalf("expected CLI exit 12, got err=%v; output=%s", cliErr, cliOutput)
	}
	if exitErr.ExitCode() != 12 {
		t.Fatalf("CLI exit=%d, want 12; output=%s", exitErr.ExitCode(), cliOutput)
	}
	if !strings.Contains(string(cliOutput), "Outcome:            inconclusive") {
		t.Fatalf("CLI missing inconclusive outcome:\\n%s", cliOutput)
	}

	t.Logf("CLI replay v3 exit 143: exit=%d", exitErr.ExitCode())
}
