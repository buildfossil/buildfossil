package docker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

func combineCleanupError(runErr, cleanupErr error, containerID string) error {
	if cleanupErr == nil {
		return runErr
	}

	return errors.Join(
		runErr,
		fmt.Errorf(
			"docker SDK: remove container %s: %w",
			containerID,
			cleanupErr,
		),
	)
}

const defaultExecutionTimeout = 30 * time.Second
const goModulesExecutionTimeout = 120 * time.Second

// RunWithSDK executes a command using the legacy Alpine runtime.
func RunWithSDK(
	ctx context.Context,
	workspace string,
	argv []string,
	user string,
	stdout, stderr io.Writer,
) (int, error) {
	return runWithSDKOptions(
		ctx,
		workspace,
		argv,
		user,
		stdout,
		stderr,
		defaultExecutionTimeout,
		nil,
	)
}

// RunWithSDKRuntime executes a command using an allowlisted runtime.
func RunWithSDKRuntime(
	ctx context.Context,
	workspace string,
	argv []string,
	user string,
	stdout, stderr io.Writer,
	runtime *RuntimeSpec,
) (int, error) {
	return runWithSDKOptions(
		ctx,
		workspace,
		argv,
		user,
		stdout,
		stderr,
		defaultExecutionTimeout,
		runtime,
	)
}

// RunWithSDKRuntimeTarget executes a command with an explicit Go target.
func RunWithSDKRuntimeTarget(
	ctx context.Context,
	workspace string,
	argv []string,
	user string,
	stdout, stderr io.Writer,
	runtime *RuntimeSpec,
	targetOS string,
	targetArch string,
) (int, error) {
	return runWithSDKOptionsTarget(
		ctx,
		workspace,
		argv,
		user,
		stdout,
		stderr,
		defaultExecutionTimeout,
		runtime,
		targetOS,
		targetArch,
	)
}

// RunWithSDKRuntimeModules executes a Go command with staged module artifacts.
// The caller must provide a trusted, immutable staging directory.
func RunWithSDKRuntimeModules(
	ctx context.Context,
	workspace string,
	argv []string,
	user string,
	stdout, stderr io.Writer,
	runtime *RuntimeSpec,
	artifactsDir string,
) (int, error) {
	if artifactsDir == "" {
		return 0, fmt.Errorf("docker SDK: empty module artifacts directory")
	}
	return runWithSDKOptionsTargetModules(
		ctx,
		workspace,
		argv,
		user,
		stdout,
		stderr,
		goModulesExecutionTimeout,
		runtime,
		"",
		"",
		artifactsDir,
	)
}

func runWithSDKTimeout(
	ctx context.Context,
	workspace string,
	argv []string,
	user string,
	stdout, stderr io.Writer,
	executionTimeout time.Duration,
) (int, error) {
	return runWithSDKOptions(
		ctx,
		workspace,
		argv,
		user,
		stdout,
		stderr,
		executionTimeout,
		nil,
	)
}

func runWithSDKOptions(
	ctx context.Context,
	workspace string,
	argv []string,
	user string,
	stdout, stderr io.Writer,
	executionTimeout time.Duration,
	runtime *RuntimeSpec,
) (int, error) {
	return runWithSDKOptionsTarget(
		ctx,
		workspace,
		argv,
		user,
		stdout,
		stderr,
		executionTimeout,
		runtime,
		"",
		"",
	)
}

func runWithSDKOptionsTarget(
	ctx context.Context,
	workspace string,
	argv []string,
	user string,
	stdout, stderr io.Writer,
	executionTimeout time.Duration,
	runtime *RuntimeSpec,
	targetOS string,
	targetArch string,
) (int, error) {
	return runWithSDKOptionsTargetModules(
		ctx,
		workspace,
		argv,
		user,
		stdout,
		stderr,
		executionTimeout,
		runtime,
		targetOS,
		targetArch,
		"",
	)
}

func runWithSDKOptionsTargetModules(
	ctx context.Context,
	workspace string,
	argv []string,
	user string,
	stdout, stderr io.Writer,
	executionTimeout time.Duration,
	runtime *RuntimeSpec,
	targetOS string,
	targetArch string,
	artifactsDir string,
) (exitCode int, runErr error) {
	if len(argv) == 0 {
		return 0, fmt.Errorf("docker SDK: empty command")
	}
	if workspace == "" {
		return 0, fmt.Errorf("docker SDK: empty workspace")
	}
	if user == "" {
		return 0, fmt.Errorf("docker SDK: empty container user")
	}

	image, err := selectReplayImage(runtime)
	if err != nil {
		return 0, err
	}

	runID, err := newRunID()
	if err != nil {
		return 0, err
	}
	dockerClient, err := NewSDKClient()
	if err != nil {
		return 0, err
	}
	defer dockerClient.Close()

	prepareCtx, cancelPrepare := context.WithTimeout(ctx, 120*time.Second)

	prepareErr := ensureReplayImage(prepareCtx, dockerClient, image)
	cancelPrepare()

	if prepareErr != nil {
		return 0, fmt.Errorf("docker SDK: prepare replay image: %w", prepareErr)
	}

	// Separate execution timeout; image preparation time does not
	// reduce the time available to the container process.
	execCtx, cancelExec := context.WithTimeout(ctx, executionTimeout)
	defer cancelExec()

	createOptions := newSecureContainerOptions(workspace, argv, user)
	createOptions.Config.Image = image
	createOptions.Config.Labels["org.buildfossil.run-id"] = runID

	if runtime != nil && runtime.Kind == "go" {
		createOptions.Config.Env = []string{
			"PATH=/usr/local/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
			"TMPDIR=/tmp",
			"GOCACHE=/gocache",
			"GOTOOLCHAIN=local",
			"GOPROXY=off",
			"GOSUMDB=off",
			"CGO_ENABLED=0",
		}

		createOptions.HostConfig.Tmpfs = map[string]string{
			"/tmp":     "rw,nosuid,nodev,size=64m,mode=1777",
			"/gocache": "rw,nosuid,nodev,size=64m,mode=1777",
		}
	}

	if artifactsDir != "" {
		if err := configureGoModuleArtifacts(
			&createOptions,
			runtime,
			artifactsDir,
		); err != nil {
			return 0, err
		}
	}

	if targetOS != "" || targetArch != "" {
		if err := configureGoTargetPlatform(
			&createOptions,
			runtime,
			targetOS,
			targetArch,
		); err != nil {
			return 0, err
		}
	}

	created, err := dockerClient.ContainerCreate(
		execCtx,
		createOptions,
	)
	if err != nil {
		return 0, fmt.Errorf("docker SDK: create container: %w", err)
	}

	defer func() {
		cleanupCtx, cancel := context.WithTimeout(
			context.Background(),
			10*time.Second,
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

		runErr = combineCleanupError(runErr, cleanupErr, created.ID)
	}()

	attached, err := dockerClient.ContainerAttach(
		execCtx,
		created.ID,
		client.ContainerAttachOptions{
			Stream: true,
			Stdin:  false,
			Stdout: true,
			Stderr: true,
			Logs:   false,
		},
	)
	if err != nil {
		return 0, fmt.Errorf("docker SDK: attach container: %w", err)
	}
	defer attached.Close()

	// Читаем потоки параллельно с выполнением контейнера.
	streamDone := make(chan error, 1)

	streamFinished := false

	defer func() {
		if !streamFinished {
			attached.Close()
			<-streamDone
		}
	}()

	go func() {
		_, streamErr := stdcopy.StdCopy(
			stdout,
			stderr,
			attached.Reader,
		)
		streamDone <- streamErr
	}()

	_, err = dockerClient.ContainerStart(
		execCtx,
		created.ID,
		client.ContainerStartOptions{},
	)
	if err != nil {
		attached.Close()
		return 0, fmt.Errorf("docker SDK: start container: %w", err)
	}

	wait := dockerClient.ContainerWait(
		execCtx,
		created.ID,
		client.ContainerWaitOptions{
			Condition: container.WaitConditionNotRunning,
		},
	)

	select {
	case response, ok := <-wait.Result:
		if !ok {
			return 0, fmt.Errorf("docker SDK: wait result channel closed")
		}
		if response.Error != nil {
			return 0, fmt.Errorf(
				"docker SDK: container wait: %s",
				response.Error.Message,
			)
		}
		exitCode = int(response.StatusCode)

	case waitErr, ok := <-wait.Error:
		if !ok {
			return 0, fmt.Errorf("docker SDK: wait error channel closed")
		}
		return 0, fmt.Errorf("docker SDK: wait container: %w", waitErr)

	case <-execCtx.Done():
		attached.Close()
		return 0, fmt.Errorf(
			"docker SDK: execution canceled: %w",
			execCtx.Err(),
		)
	}

	// Важно: ContainerWait не гарантирует, что все данные
	// уже прочитаны из Attach-соединения.
	select {
	case streamErr := <-streamDone:
		streamFinished = true

		if streamErr != nil {
			return 0, fmt.Errorf(
				"docker SDK: read container streams: %w",
				streamErr,
			)
		}

	case <-execCtx.Done():
		attached.Close()
		return 0, fmt.Errorf(
			"docker SDK: stream canceled: %w",
			execCtx.Err(),
		)
	}

	return exitCode, nil
}
