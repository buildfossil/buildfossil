package docker

import (
	"context"
	"fmt"
	"io"
	"os/exec"
)

const image = "alpine:3.20"

func Run(
	ctx context.Context,
	workspace string,
	argv []string,
	stdout, stderr io.Writer,
) (int, error) {
	if len(argv) == 0 {
		return 0, fmt.Errorf("docker: empty command")
	}

	args := []string{
		"run",
		"--rm",
		"--platform", "linux/amd64",
		"--network", "none",
		"--cap-drop", "ALL",
		"--security-opt", "no-new-privileges",
		"--pids-limit", "64",
		"--memory", "256m",
		"--cpus", "1",
		"--read-only",
		"--user", "65534:65534",
		"--mount", "type=bind,source=" + workspace + ",target=/workspace,readonly",
		"--workdir", "/workspace",
		image,
	}

	args = append(args, argv...)

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err := cmd.Run()
	if err == nil {
		return 0, nil
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		code := exitErr.ExitCode()

		if code == 125 || code == 126 || code == 127 {
			return code, fmt.Errorf(
				"docker: exit code %d is ambiguous; replay inconclusive: %w",
				code,
				err,
			)
		}

		return code, nil
	}

	return 0, fmt.Errorf("docker: run container: %w", err)
}
