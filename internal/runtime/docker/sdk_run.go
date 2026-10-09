package docker

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

// RunWithSDK executes a command in an isolated Docker container.
// stdout and stderr contain only the container process output.
func RunWithSDK(
	ctx context.Context,
	workspace string,
	argv []string,
	user string,
	stdout, stderr io.Writer,
) (int, error) {
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

	created, err := dockerClient.ContainerCreate(
		ctx,
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

		_, _ = dockerClient.ContainerRemove(
			cleanupCtx,
			created.ID,
			client.ContainerRemoveOptions{
				Force:         true,
				RemoveVolumes: true,
			},
		)
	}()

	_, err = dockerClient.ContainerStart(
		ctx,
		created.ID,
		client.ContainerStartOptions{},
	)
	if err != nil {
		return 0, fmt.Errorf("docker SDK: start container: %w", err)
	}

	wait := dockerClient.ContainerWait(
		ctx,
		created.ID,
		client.ContainerWaitOptions{
			Condition: container.WaitConditionNotRunning,
		},
	)

	var exitCode int

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

	case err, ok := <-wait.Error:
		if !ok {
			return 0, fmt.Errorf("docker SDK: wait error channel closed")
		}
		return 0, fmt.Errorf("docker SDK: wait container: %w", err)

	case <-ctx.Done():
		return 0, fmt.Errorf("docker SDK: execution canceled: %w", ctx.Err())
	}

	logs, err := dockerClient.ContainerLogs(
		ctx,
		created.ID,
		client.ContainerLogsOptions{
			ShowStdout: true,
			ShowStderr: true,
		},
	)
	if err != nil {
		return 0, fmt.Errorf("docker SDK: retrieve logs: %w", err)
	}
	defer logs.Close()

	if _, err := stdcopy.StdCopy(stdout, stderr, logs); err != nil {
		return 0, fmt.Errorf("docker SDK: decode container streams: %w", err)
	}

	return exitCode, nil
}
