package docker

import (
	"context"
	"fmt"
)

// ensureLegacyImage prepares the legacy replay image using Docker SDK.
// Image inspection and pulling are kept separate from the replayed
// command's stdout and stderr.
func ensureLegacyImage(ctx context.Context) error {
	dockerClient, err := NewSDKClient()
	if err != nil {
		return fmt.Errorf("docker: initialize legacy image client: %w", err)
	}
	defer dockerClient.Close()

	if err := ensureReplayImage(ctx, dockerClient, image); err != nil {
		return fmt.Errorf("docker: prepare legacy replay image: %w", err)
	}

	return nil
}
