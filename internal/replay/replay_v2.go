package replay

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/buildfossil/buildfossil/internal/capsule"
	"github.com/buildfossil/buildfossil/internal/runtime/docker"
)

// RunV2WithOptions replays a verified schema-v2 capsule.
//
// Experimental: Docker bind-mount permissions have not yet
// been validated on a native Linux host.
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

	code, err := docker.RunWithUser(
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

	fmt.Fprintf(
		os.Stderr,
		"DEBUG replay v2: original stderr=%q, replay stderr=%q, original truncated=%t, replay truncated=%t\n",
		verified.Manifest.Execution.Stderr,
		string(stderr.Bytes()),
		verified.Manifest.Execution.StderrTruncated,
		stderr.Truncated(),
	)

	outcome, err := CompareFailure(
		verified.Manifest.Execution.ExitCode,
		verified.Manifest.Execution.Stderr,
		verified.Manifest.Execution.StderrTruncated,
		code,
		stderr.Bytes(),
		stderr.Truncated(),
	)
	if outcome != OutcomeReproduced {
		original := []byte(verified.Manifest.Execution.Stderr)
		replayed := stderr.Bytes()

		fmt.Fprintf(
			os.Stderr,
			"DEBUG replay comparison: original_len=%d original_sha256=%x original_truncated=%t replay_len=%d replay_sha256=%x replay_truncated=%t original_exit=%d replay_exit=%d outcome=%s\n",
			len(original),
			sha256.Sum256(original),
			verified.Manifest.Execution.StderrTruncated,
			len(replayed),
			sha256.Sum256(replayed),
			stderr.Truncated(),
			verified.Manifest.Execution.ExitCode,
			code,
			outcome,
		)
	}
	if err != nil {
		return result, fmt.Errorf("replay: compare failure: %w", err)
	}

	return Result{
		OriginalExitCode: verified.Manifest.Execution.ExitCode,
		ReplayExitCode:   code,
		Outcome:          outcome,
	}, nil
}
