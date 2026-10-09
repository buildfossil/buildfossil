package docker

import (
	"context"
	"fmt"

	"github.com/moby/moby/client"
)

// NewSDKClient creates a Docker Engine API client using
// the user's Docker environment configuration.
func NewSDKClient() (*client.Client, error) {
	dockerClient, err := client.New(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("initialize Docker SDK client: %w", err)
	}

	return dockerClient, nil
}

// CheckSDKConnection verifies that Docker Engine is reachable.
func CheckSDKConnection(ctx context.Context) error {
	dockerClient, err := NewSDKClient()
	if err != nil {
		return err
	}
	defer dockerClient.Close()

	if _, err := dockerClient.Ping(ctx, client.PingOptions{}); err != nil {
		return fmt.Errorf("connect to Docker Engine: %w", err)
	}

	return nil
}
