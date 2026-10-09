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

// RunWithSDK executes a command in an isolated Docker container.
// stdout and stderr contain only the container process output.
func RunWithSDK(
	ctx context.Context,
	workspace string,
	argv []string,
	user string,
	stdout, stderr io.Writer,
) (int, error) {
	return runWithSDKTimeout(
		ctx,
		workspace,
		argv,
		user,
		stdout,
		stderr,
		defaultExecutionTimeout,
	)
}

func runWithSDKTimeout(
	ctx context.Context,
	workspace string,
	argv []string,
	user string,
	stdout, stderr io.Writer,
	executionTimeout time.Duration,
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

	dockerClient, err := NewSDKClient()
	if err != nil {
		return 0, err
	}
	defer dockerClient.Close()

	prepareCtx, cancelPrepare := context.WithTimeout(ctx, 120*time.Second)

	prepareErr := ensureReplayImage(prepareCtx, dockerClient)
	cancelPrepare()

	if prepareErr != nil {
		return 0, fmt.Errorf("docker SDK: prepare replay image: %w", prepareErr)
	}

	// Separate execution timeout; image preparation time does not
	// reduce the time available to the container process.
	execCtx, cancelExec := context.WithTimeout(ctx, executionTimeout)
	defer cancelExec()

	created, err := dockerClient.ContainerCreate(
		execCtx,
		newSecureContainerOptions(workspace, argv, user),
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
