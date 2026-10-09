package replay

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/buildfossil/buildfossil/internal/capsule"
	"github.com/buildfossil/buildfossil/internal/runtime/docker"
)

type Result struct {
	OriginalExitCode int
	ReplayExitCode   int
	Outcome          Outcome
}

func Run(ctx context.Context, capsulePath string) (Result, error) {
	var result Result

	verified, err := capsule.ReadVerified(capsulePath)
	if err != nil {
		return result, fmt.Errorf("replay: verify capsule: %w", err)
	}

	platform := verified.Manifest.Platform

	if platform.OS != "linux" || platform.Architecture != "amd64" {
		return result, fmt.Errorf(
			"replay: unsupported source platform %s/%s; expected linux/amd64",
			platform.OS,
			platform.Architecture,
		)
	}

	// Experimental replay supports exactly one known command.
	argv := verified.Manifest.Execution.Argv
	if len(argv) != 3 ||
		argv[0] != "/bin/sh" ||
		argv[1] != "-c" ||
		argv[2] != "cat fixture.txt >&2; exit 17" {
		return result, fmt.Errorf("replay: unsupported command")
	}

	workspace, err := os.MkdirTemp("", "buildfossil-replay-*")
	if err != nil {
		return result, fmt.Errorf("replay: create workspace: %w", err)
	}
	defer os.RemoveAll(workspace)

	if err := capsule.RestoreWorkspace(workspace, verified); err != nil {
		return result, fmt.Errorf("replay: restore workspace: %w", err)
	}

	fixture := filepath.Join(workspace, "fixture.txt")
	if err := os.Chmod(fixture, 0644); err != nil {
		return result, fmt.Errorf("replay: prepare permissions: %w", err)
	}

	var stderr boundedOutput

	code, err := docker.Run(
		ctx,
		workspace,
		argv,
		io.Discard,
		io.MultiWriter(os.Stderr, &stderr),
	)
	if err != nil {
		return result, fmt.Errorf("replay: Docker execution: %w", err)
	}

	outcome, err := CompareFailure(
		verified.Manifest.Execution.ExitCode,
		verified.Manifest.Execution.Stderr,
		verified.Manifest.Execution.StderrTruncated,
		code,
		stderr.Bytes(),
		stderr.Truncated(),
	)
	if err != nil {
		return result, fmt.Errorf("replay: compare failure: %w", err)
	}

	result = Result{
		OriginalExitCode: verified.Manifest.Execution.ExitCode,
		ReplayExitCode:   code,
		Outcome:          outcome,
	}

	return result, nil
}
