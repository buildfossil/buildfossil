package docker

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/moby/moby/client"
)

// SnapshotVolumeCandidate is a Docker volume whose name and metadata
// match the REW snapshot convention. It is not proof of ownership,
// nor evidence that the volume is orphaned.
type SnapshotVolumeCandidate struct {
	Name string
	Kind string
}

func isLowerHexID(value string) bool {
	if len(value) != 32 {
		return false
	}

	for _, c := range value {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}

	return true
}

func isSnapshotVolumeCandidate(
	name string,
	labels map[string]string,
) (string, bool) {
	const prefix = "rew-snapshot-"

	if !strings.HasPrefix(name, prefix) ||
		!isLowerHexID(strings.TrimPrefix(name, prefix)) {
		return "", false
	}

	if labels["org.rew.component"] != "snapshot" {
		return "", false
	}

	kind := labels["org.rew.kind"]
	if kind != "workspace" && kind != "module-cache" {
		return "", false
	}

	if !isLowerHexID(labels["org.rew.ownership-token"]) {
		return "", false
	}

	return kind, true
}

// ListSnapshotVolumeCandidates reads Docker volume metadata.
// It never creates, modifies, or deletes Docker resources.
func ListSnapshotVolumeCandidates(
	ctx context.Context,
	dockerClient *client.Client,
) ([]SnapshotVolumeCandidate, error) {
	if dockerClient == nil {
		return nil, fmt.Errorf("docker SDK: nil client")
	}

	result, err := dockerClient.VolumeList(
		ctx,
		client.VolumeListOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("docker SDK: list snapshot volumes: %w", err)
	}
	if len(result.Warnings) != 0 {
		return nil, fmt.Errorf(
			"docker SDK: incomplete volume inspection: %s",
			strings.Join(result.Warnings, "; "),
		)
	}

	var candidates []SnapshotVolumeCandidate

	for _, volume := range result.Items {
		kind, ok := isSnapshotVolumeCandidate(
			volume.Name,
			volume.Labels,
		)
		if !ok {
			continue
		}

		candidates = append(candidates, SnapshotVolumeCandidate{
			Name: volume.Name,
			Kind: kind,
		})
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Name < candidates[j].Name
	})

	return candidates, nil
}
