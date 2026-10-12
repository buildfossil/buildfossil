package replay

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/buildfossil/buildfossil/internal/capsule"
	"github.com/buildfossil/buildfossil/internal/runtime/docker"
)

// RunV3WithOptions replays a verified schema-v3 capsule with
// up to 128 selected Go modules, with at most 64 complete module ZIPs, without network access.
//
// Experimental: uses Docker Exec, which is not equivalent to PID 1.
func RunV3WithOptions(
	ctx context.Context,
	capsulePath string,
	options Options,
) (result Result, runErr error) {

	if options.MemoryBytes != 0 {
		switch options.MemoryBytes {
		case 256 << 20, 512 << 20, 1 << 30, 2 << 30:
		default:
			return result, fmt.Errorf(
				"replay: invalid v3 memory limit: %d bytes",
				options.MemoryBytes,
			)
		}
	}

	verified, err := capsule.ReadVerifiedToStoreV3(capsulePath)
	if err != nil {
		return result, fmt.Errorf("replay: verify v3 capsule: %w", err)
	}

	// Keep verified artifacts available until snapshot preparation
	// and Docker execution have completed.
	defer func() {
		if err := verified.Close(); err != nil {
			runErr = errors.Join(
				runErr,
				fmt.Errorf("replay: cleanup artifact store: %w", err),
			)
		}
	}()

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

	snapshots, err := prepareSnapshotFilesFromStoreV3(verified)
	if err != nil {
		return result, fmt.Errorf(
			"replay: prepare verified snapshots: %w", err,
		)
	}
	defer func() {
		runErr = closeSnapshotFilesV3(snapshots, runErr)
	}()

	selectedModules := capsule.EffectiveGoSelectedModulesV3(
		verified.Manifest,
	)

	expectedModules := make(
		[]docker.ExpectedGoModule,
		0,
		len(selectedModules),
	)
	for _, mod := range selectedModules {
		expectedModules = append(expectedModules, docker.ExpectedGoModule{
			Path:    mod.Path,
			Version: mod.Version,
		})
	}

	materializedModules := make(
		[]docker.ExpectedGoModule,
		0,
		len(verified.Manifest.GoModules),
	)
	for _, mod := range verified.Manifest.GoModules {
		materializedModules = append(
			materializedModules,
			docker.ExpectedGoModule{
				Path:    mod.Path,
				Version: mod.Version,
			},
		)
	}

	var stderr boundedOutput

	code, err := docker.RunWithSDKRuntimeSnapshotExecSelectiveReadersV3(
		ctx,
		io.NopCloser(snapshots.workspace),
		io.NopCloser(snapshots.modules),
		verified.Manifest.Execution.Argv,
		expectedModules,
		materializedModules,
		options.MemoryBytes,
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

	outcome, err := compareFailureV3(
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
		Difference: DiagnoseFailureDifferenceV3(
			outcome,
			verified.Manifest.Execution.ExitCode,
			verified.Manifest.Execution.Stderr,
			code,
			stderr.Bytes(),
		),
	}, nil
}
