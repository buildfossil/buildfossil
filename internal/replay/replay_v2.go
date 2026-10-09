package replay

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/buildfossil/buildfossil/internal/capsule"
	"github.com/buildfossil/buildfossil/internal/runtime/docker"
)

// RunV2WithOptions replays a verified schema-v2 capsule.
//
// Experimental: Docker bind-mount behavior has not been validated
// across all Linux and Docker configurations.
func RunV2WithOptions(
	ctx context.Context,
	capsulePath string,
	options Options,
) (Result, error) {
	var result Result

	verified, err := capsule.ReadVerifiedV2(capsulePath)
	if err != nil {
		return result, fmt.Errorf("replay: verify v2 capsule: %w", err)
	}

	platform := verified.Manifest.Platform
	if platform.OS != "linux" || platform.Architecture != "amd64" {
		return result, fmt.Errorf(
			"replay: unsupported source platform %s/%s; expected linux/amd64",
			platform.OS,
			platform.Architecture,
		)
	}

	if !options.AllowArbitraryCommand {
		return result, fmt.Errorf(
			"replay: v2 command execution requires explicit opt-in",
		)
	}

	uid := os.Geteuid()
	gid := os.Getegid()

	if uid == 0 {
		return result, fmt.Errorf(
			"replay: v2 refuses to run Docker as root",
		)
	}

	containerUser := strconv.Itoa(uid) + ":" + strconv.Itoa(gid)
	argv := verified.Manifest.Execution.Argv

	workspace, err := os.MkdirTemp("", "buildfossil-replay-v2-*")
	if err != nil {
		return result, fmt.Errorf("replay: create workspace: %w", err)
	}
	defer os.RemoveAll(workspace)

	if err := capsule.RestoreWorkspaceV2(workspace, verified); err != nil {
		return result, fmt.Errorf("replay: restore v2 workspace: %w", err)
	}

	var stderr boundedOutput

	code, err := docker.RunWithSDK(
		ctx,
		workspace,
		argv,
		containerUser,
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

	return Result{
		OriginalExitCode: verified.Manifest.Execution.ExitCode,
		ReplayExitCode:   code,
		Outcome:          outcome,
	}, nil
}
