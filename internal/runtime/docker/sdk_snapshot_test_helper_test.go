package docker

import (
	"context"
	"testing"
	"time"

	"github.com/moby/moby/client"
)

// ensureSnapshotTestImage ensures the pinned Go image is available
// before a Docker snapshot integration test starts.
func ensureSnapshotTestImage(
	t *testing.T,
	dockerClient *client.Client,
) {
	t.Helper()

	ctx, cancel := context.WithTimeout(
		context.Background(),
		120*time.Second,
	)
	defer cancel()

	if err := ensureReplayImage(ctx, dockerClient, goReplayImage); err != nil {
		t.Fatalf("prepare snapshot test image: %v", err)
	}
}
