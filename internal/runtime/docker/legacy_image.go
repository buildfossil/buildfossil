package docker

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// ensureLegacyImage separates image acquisition from the replayed
// command stderr.
func ensureLegacyImage(ctx context.Context) error {
	inspect := exec.CommandContext(ctx, "docker", "image", "inspect", image)

	if err := inspect.Run(); err == nil {
		return nil
	}

	// Verify that the daemon is reachable before attempting a pull.
	info := exec.CommandContext(ctx, "docker", "info", "--format", "{{.ServerVersion}}")
	if output, err := info.CombinedOutput(); err != nil {
		return fmt.Errorf(
			"docker: daemon unavailable: %w: %s",
			err,
			strings.TrimSpace(string(output)),
		)
	}

	pull := exec.CommandContext(
		ctx,
		"docker", "pull",
		"--platform", "linux/amd64",
		image,
	)

	output, err := pull.CombinedOutput()
	if err != nil {
		return fmt.Errorf(
			"docker: prepare legacy replay image: %w: %s",
			err,
			strings.TrimSpace(string(output)),
		)
	}

	return nil
}
