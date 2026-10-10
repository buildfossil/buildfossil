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

// RunV3WithOptions replays a verified schema-v3 capsule with
// zero or one embedded Go module, without network access.
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

	if len(verified.Manifest.GoModules) > capsule.MaxGoModulesV3 {
		return result, fmt.Errorf("replay: too many Go modules")
	}

	uid := os.Geteuid()
	gid := os.Getegid()

	if uid == 0 {
		return result, fmt.Errorf("replay: refuses to run Docker as root")
	}

	containerUser := strconv.Itoa(uid) + ":" + strconv.Itoa(gid)

	workspaceTAR, err := capsule.BuildWorkspaceSnapshotV3(verified)
	if err != nil {
		return result, fmt.Errorf(
			"replay: build verified workspace snapshot: %w",
			err,
		)
	}

	moduleTAR, err := capsule.BuildGoModuleSnapshotV3(verified)
	if err != nil {
		return result, fmt.Errorf(
			"replay: build verified Go module snapshot: %w",
			err,
		)
	}

	var stderr boundedOutput

	code, err := docker.RunWithSDKRuntimeSnapshotExec(
		ctx,
		workspaceTAR,
		moduleTAR,
		verified.Manifest.Execution.Argv,
		containerUser,
		io.Discard,
		&stderr,
		&docker.RuntimeSpec{
			Kind:    verified.Manifest.Runtime.Kind,
			Version: verified.Manifest.Runtime.Version,
		},
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
