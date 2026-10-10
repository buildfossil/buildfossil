package replay

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/buildfossil/buildfossil/internal/capsule"
	"github.com/buildfossil/buildfossil/internal/runtime/docker"
)

// RunV3WithOptions replays a verified schema-v3 capsule with
// one embedded Go module, without network access.
//
// Experimental: uses Docker Exec, which is not equivalent to PID 1.
func RunV3WithOptions(
	ctx context.Context,
	capsulePath string,
	options Options,
) (Result, error) {
	var result Result

	verified, err := capsule.ReadVerifiedV3(capsulePath)
	if err != nil {
		return result, fmt.Errorf("replay: verify v3 capsule: %w", err)
	}

	if err := validateReplayPlatform(verified.Manifest); err != nil {
		return result, err
	}

	if !options.AllowArbitraryCommand {
		return result, fmt.Errorf(
			"replay: v3 command execution requires explicit opt-in",
		)
	}

	if verified.Manifest.Runtime == nil ||
		verified.Manifest.Runtime.Kind != "go" ||
		verified.Manifest.Runtime.Version != "1.26.6" {
		return result, fmt.Errorf("replay: unsupported v3 runtime")
	}

	if len(verified.Manifest.GoModules) != 1 {
		return result, fmt.Errorf("replay: expected one Go module")
	}

	uid := os.Geteuid()
	gid := os.Getegid()

	if uid == 0 {
		return result, fmt.Errorf("replay: refuses to run Docker as root")
	}

	containerUser := strconv.Itoa(uid) + ":" + strconv.Itoa(gid)

	replayRoot, err := os.MkdirTemp("", "buildfossil-replay-v3-*")
	if err != nil {
		return result, fmt.Errorf("replay: create temporary directory: %w", err)
	}
	defer os.RemoveAll(replayRoot)

	workspace := filepath.Join(replayRoot, "workspace")
	artifacts := filepath.Join(replayRoot, "module-artifacts")
	cacheRoot := filepath.Join(replayRoot, "module-cache")

	for _, path := range []string{workspace, artifacts, cacheRoot} {
		if err := os.Mkdir(path, 0700); err != nil {
			return result, fmt.Errorf("replay: create staging directory: %w", err)
		}
	}

	if err := capsule.RestoreWorkspaceV3(
		workspace,
		artifacts,
		verified,
	); err != nil {
		return result, fmt.Errorf("replay: restore workspace: %w", err)
	}

	if err := capsule.PrepareGoModuleCacheV3(
		cacheRoot,
		verified,
	); err != nil {
		return result, fmt.Errorf("replay: prepare offline cache: %w", err)
	}

	if err := capsule.VerifyStagedWorkspaceV3(
		workspace,
		verified.Manifest,
	); err != nil {
		return result, fmt.Errorf(
			"replay: verify staged workspace: %w",
			err,
		)
	}

	var stderr boundedOutput

	code, err := docker.RunWithSDKRuntimeOfflineExec(
		ctx,
		workspace,
		verified.Manifest.Execution.Argv,
		containerUser,
		io.Discard,
		&stderr,
		&docker.RuntimeSpec{
			Kind:    verified.Manifest.Runtime.Kind,
			Version: verified.Manifest.Runtime.Version,
		},
		cacheRoot,
	)
	if err != nil {
		return result, fmt.Errorf("replay: offline Docker execution: %w", err)
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
