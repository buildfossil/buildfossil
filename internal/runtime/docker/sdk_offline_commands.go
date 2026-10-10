package docker

import (
	"context"
	"fmt"
	"io"

	"github.com/moby/moby/client"
)

// runOfflineGoCommands executes an offline Go command inside
// an already running and prepared container.
//
// The caller owns the container lifecycle.
func runOfflineGoCommands(
	ctx context.Context,
	dockerClient *client.Client,
	containerID string,
	user string,
	argv []string,
	stdout, stderr io.Writer,
) (int, error) {
	if len(argv) == 0 {
		return 0, fmt.Errorf("docker SDK: empty offline argv")
	}

	prepareCode, _, prepareStderr, err := executeInContainer(
		ctx,
		dockerClient,
		containerID,
		user,
		[]string{
			"/bin/sh",
			"-ec",
			"cp -R /module-cache-source/. /gomodcache/",
		},
	)
	if err != nil {
		return 0, fmt.Errorf(
			"docker SDK: prepare module cache: %w", err,
		)
	}

	if prepareCode != 0 {
		return 0, fmt.Errorf(
			"docker SDK: module cache preparation failed (exit %d): %s",
			prepareCode,
			string(prepareStderr),
		)
	}

	moduleCode, _, moduleStderr, err := executeInContainer(
		ctx,
		dockerClient,
		containerID,
		user,
		[]string{"go", "mod", "download"},
	)
	if err != nil {
		return 0, fmt.Errorf(
			"docker SDK: offline module materialization: %w",
			err,
		)
	}

	if moduleCode != 0 {
		return 0, fmt.Errorf(
			"docker SDK: offline module materialization failed (exit %d): %s",
			moduleCode,
			string(moduleStderr),
		)
	}

	if err := ctx.Err(); err != nil {
		return 0, fmt.Errorf(
			"docker SDK: offline module preparation canceled: %w",
			err,
		)
	}

	code, out, errOut, err := executeInContainer(
		ctx,
		dockerClient,
		containerID,
		user,
		argv,
	)
	if err != nil {
		return 0, fmt.Errorf("docker SDK: offline execution: %w", err)
	}

	if stdout != nil {
		if _, err := stdout.Write(out); err != nil {
			return 0, fmt.Errorf("docker SDK: stdout: %w", err)
		}
	}

	if stderr != nil {
		if _, err := stderr.Write(errOut); err != nil {
			return 0, fmt.Errorf("docker SDK: stderr: %w", err)
		}
	}

	return code, nil
}
