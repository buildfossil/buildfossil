package docker

import (
	"context"
	"fmt"

	"github.com/containerd/errdefs"
	"github.com/moby/moby/client"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// ensureReplayImage makes the pinned replay image available locally.
// Registry operations are kept separate from container stderr.
func ensureReplayImage(ctx context.Context, dockerClient *client.Client) error {
	_, err := dockerClient.ImageInspect(ctx, replayImage)
	if err == nil {
		return nil
	}

	if !errdefs.IsNotFound(err) {
		return fmt.Errorf("docker SDK: inspect replay image: %w", err)
	}

	pull, err := dockerClient.ImagePull(
		ctx,
		replayImage,
		client.ImagePullOptions{
			Platforms: []ocispec.Platform{
				{
					OS:           "linux",
					Architecture: "amd64",
				},
			},
		},
	)
	if err != nil {
		return fmt.Errorf("docker SDK: pull replay image: %w", err)
	}
	defer pull.Close()

	if err := pull.Wait(ctx); err != nil {
		return fmt.Errorf("docker SDK: complete image pull: %w", err)
	}

	// Verify that Docker Engine can resolve the pinned image
	// after downloading it.
	if _, err := dockerClient.ImageInspect(ctx, replayImage); err != nil {
		return fmt.Errorf("docker SDK: verify pulled image: %w", err)
	}

	return nil
}
