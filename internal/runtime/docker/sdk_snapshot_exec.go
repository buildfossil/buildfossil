package docker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/client"
)

// RunWithSDKRuntimeSnapshotExec executes offline Go commands
// using Docker-managed volumes rather than host bind mounts.
//
// The caller supplies TAR archives constructed from verified
// capsule data. The executor owns all Docker resources.
func RunWithSDKRuntimeSnapshotExec(
	ctx context.Context,
	workspaceTAR []byte,
	moduleTAR []byte,
	argv []string,
	user string,
	stdout, stderr io.Writer,
	runtime *RuntimeSpec,
) (exitCode int, runErr error) {
	return runWithSDKRuntimeSnapshotExec(
		ctx, workspaceTAR, moduleTAR, argv,
		nil, false, 0, user, stdout, stderr, runtime,
	)
}

// RunWithSDKRuntimeSnapshotExecVerified requires an exact offline
// Go module graph match before executing the captured command.
func RunWithSDKRuntimeSnapshotExecVerified(
	ctx context.Context,
	workspaceTAR []byte,
	moduleTAR []byte,
	argv []string,
	expected []ExpectedGoModule,
	user string,
	stdout, stderr io.Writer,
	runtime *RuntimeSpec,
) (int, error) {
	return RunWithSDKRuntimeSnapshotExecVerifiedWithMemory(
		ctx, workspaceTAR, moduleTAR, argv,
		expected, 0, user, stdout, stderr, runtime,
	)
}

// RunWithSDKRuntimeSnapshotExecVerifiedWithMemory runs schema-v3
// replay with an explicitly bounded execution-container memory limit.
func RunWithSDKRuntimeSnapshotExecVerifiedWithMemory(
	ctx context.Context,
	workspaceTAR []byte,
	moduleTAR []byte,
	argv []string,
	expected []ExpectedGoModule,
	memoryBytes int64,
	user string,
	stdout, stderr io.Writer,
	runtime *RuntimeSpec,
) (int, error) {
	return runWithSDKRuntimeSnapshotExec(
		ctx, workspaceTAR, moduleTAR, argv,
		expected, true, memoryBytes, user, stdout, stderr, runtime,
	)
}

// RunWithSDKRuntimeSnapshotExecSelectiveV3 materializes only
// modules with complete ZIP artifacts, while verifying the full
// selected Go module graph before executing the captured command.
func RunWithSDKRuntimeSnapshotExecSelectiveV3(
	ctx context.Context,
	workspaceTAR []byte,
	moduleTAR []byte,
	argv []string,
	expected []ExpectedGoModule,
	materialized []ExpectedGoModule,
	memoryBytes int64,
	user string,
	stdout, stderr io.Writer,
	runtime *RuntimeSpec,
) (int, error) {
	downloadArgs, err := selectiveGoModuleDownloadArgsV3(
		expected, materialized,
	)
	if err != nil {
		return 0, err
	}

	return runWithSDKRuntimeSnapshotExecSelective(
		ctx, workspaceTAR, moduleTAR, argv,
		expected, true, memoryBytes, user,
		stdout, stderr, runtime, downloadArgs, true,
	)
}

func runWithSDKRuntimeSnapshotExec(
	ctx context.Context,
	workspaceTAR []byte,
	moduleTAR []byte,
	argv []string,
	expected []ExpectedGoModule,
	verifyGraph bool,
	memoryBytes int64,
	user string,
	stdout, stderr io.Writer,
	runtime *RuntimeSpec,
) (exitCode int, runErr error) {
	return runWithSDKRuntimeSnapshotExecSelective(
		ctx, workspaceTAR, moduleTAR, argv,
		expected, verifyGraph, memoryBytes, user,
		stdout, stderr, runtime, nil, false,
	)
}

func runWithSDKRuntimeSnapshotExecSelective(
	ctx context.Context,
	workspaceTAR []byte,
	moduleTAR []byte,
	argv []string,
	expected []ExpectedGoModule,
	verifyGraph bool,
	memoryBytes int64,
	user string,
	stdout, stderr io.Writer,
	runtime *RuntimeSpec,
	downloadArgs []string,
	selective bool,
) (exitCode int, runErr error) {
	if len(argv) == 0 {
		return 0, fmt.Errorf("docker SDK: empty command")
	}

	if err := validateReplayMemoryV3(memoryBytes); err != nil {
		return 0, err
	}

	if runtime == nil ||
		runtime.Kind != "go" ||
		runtime.Version != "1.26.6" {
		return 0, fmt.Errorf("docker SDK: unsupported offline Go runtime")
	}

	parts := strings.Split(user, ":")
	if len(parts) != 2 {
		return 0, fmt.Errorf("docker SDK: expected numeric UID:GID")
	}

	uid, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil || uid == 0 {
		return 0, fmt.Errorf("docker SDK: invalid or root UID")
	}

	if _, err := strconv.ParseUint(parts[1], 10, 32); err != nil {
		return 0, fmt.Errorf("docker SDK: invalid GID")
	}

	if len(workspaceTAR) == 0 || len(moduleTAR) == 0 {
		return 0, fmt.Errorf("docker SDK: empty snapshot TAR")
	}

	if err := ctx.Err(); err != nil {
		return 0, err
	}

	image, err := selectReplayImage(runtime)
	if err != nil {
		return 0, err
	}

	dockerClient, err := NewSDKClient()
	if err != nil {
		return 0, err
	}
	defer dockerClient.Close()

	prepareCtx, cancelPrepare := context.WithTimeout(
		ctx,
		120*time.Second,
	)

	prepareErr := ensureReplayImage(
		prepareCtx,
		dockerClient,
		image,
	)
	cancelPrepare()

	if prepareErr != nil {
		return 0, fmt.Errorf(
			"docker SDK: prepare image: %w",
			prepareErr,
		)
	}

	execCtx, cancelExec := context.WithTimeout(
		ctx,
		goModulesExecutionTimeout,
	)
	defer cancelExec()

	volumes, err := CreateSnapshotVolumes(
		execCtx,
		dockerClient,
		image,
		workspaceTAR,
		moduleTAR,
	)
	if err != nil {
		return 0, fmt.Errorf(
			"docker SDK: prepare snapshot volumes: %w",
			err,
		)
	}

	// Registered first: runs after execution-container cleanup.
	defer func() {
		if cleanupErr := volumes.Close(); cleanupErr != nil {
			runErr = errors.Join(
				runErr,
				fmt.Errorf(
					"docker SDK: cleanup snapshot volumes: %w",
					cleanupErr,
				),
			)
		}
	}()

	options := newSnapshotExecutionOptions(
		image,
		user,
		volumes,
	)
	if err := applyReplayMemoryV3(&options, memoryBytes); err != nil {
		return 0, err
	}
	created, err := dockerClient.ContainerCreate(
		execCtx,
		options,
	)
	if err != nil {
		return 0, fmt.Errorf(
			"docker SDK: create snapshot container: %w",
			err,
		)
	}

	// Registered second: removes the container before its volumes.
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(
			context.Background(),
			15*time.Second,
		)
		defer cancel()

		_, cleanupErr := dockerClient.ContainerRemove(
			cleanupCtx,
			created.ID,
			client.ContainerRemoveOptions{
				Force:         true,
				RemoveVolumes: true,
			},
		)

		if cleanupErr != nil {
			runErr = errors.Join(
				runErr,
				fmt.Errorf(
					"docker SDK: cleanup snapshot container: %w",
					cleanupErr,
				),
			)
		}
	}()

	if _, err := dockerClient.ContainerStart(
		execCtx,
		created.ID,
		client.ContainerStartOptions{},
	); err != nil {
		return 0, fmt.Errorf(
			"docker SDK: start snapshot container: %w",
			err,
		)
	}

	var code int

	if selective {
		code, err = runOfflineGoCommandsWithGraphSelective(
			execCtx, dockerClient, created.ID, user,
			argv, expected, verifyGraph, downloadArgs, stdout, stderr,
		)
	} else {
		code, err = runOfflineGoCommandsWithGraph(
			execCtx, dockerClient, created.ID, user,
			argv, expected, verifyGraph, stdout, stderr,
		)
	}
	if err != nil {
		return 0, err
	}

	return code, nil
}

func snapshotExecutionMounts(volumes *SnapshotVolumes) []mount.Mount {
	return []mount.Mount{
		{
			Type:     mount.TypeVolume,
			Source:   volumes.Workspace,
			Target:   "/workspace",
			ReadOnly: true,
			VolumeOptions: &mount.VolumeOptions{
				NoCopy: true,
			},
		},
		{
			Type:     mount.TypeVolume,
			Source:   volumes.ModuleCache,
			Target:   "/module-cache-source",
			ReadOnly: true,
			VolumeOptions: &mount.VolumeOptions{
				NoCopy: true,
			},
		},
	}
}

func newSnapshotExecutionOptions(
	image string,
	user string,
	volumes *SnapshotVolumes,
) client.ContainerCreateOptions {
	options := newSecureContainerOptions(
		"",
		[]string{
			"/bin/sh",
			"-c",
			"while :; do sleep 60; done",
		},
		user,
	)

	options.Config.Image = image

	options.Config.Env = []string{
		"PATH=/usr/local/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"TMPDIR=/tmp",
		"GOCACHE=/gocache",
		"GOMODCACHE=/gomodcache",
		"GOWORK=off",
		"GOFLAGS=",
		"GOTOOLCHAIN=local",
		"GOPROXY=off",
		"GOSUMDB=off",
		"CGO_ENABLED=0",
	}

	options.HostConfig.Tmpfs = map[string]string{
		"/tmp":        "rw,nosuid,nodev,size=512m,mode=1777",
		"/gocache":    "rw,nosuid,nodev,size=256m,mode=1777",
		"/gomodcache": "rw,nosuid,nodev,size=64m,mode=1777",
	}

	// Replace the inherited bind mount, never append to it.
	options.HostConfig.Mounts = snapshotExecutionMounts(volumes)

	return options
}
