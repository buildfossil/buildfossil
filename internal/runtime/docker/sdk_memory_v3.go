package docker

import (
	"fmt"

	"github.com/moby/moby/client"
)

const (
	defaultReplayMemoryV3 int64 = 256 << 20
	maxReplayMemoryV3     int64 = 2 << 30
)

func validateReplayMemoryV3(memoryBytes int64) error {
	switch memoryBytes {
	case 0, 256 << 20, 512 << 20, 1 << 30, 2 << 30:
		return nil
	default:
		return fmt.Errorf(
			"docker SDK: unsupported v3 replay memory limit: %d",
			memoryBytes,
		)
	}
}

func applyReplayMemoryV3(
	options *client.ContainerCreateOptions,
	memoryBytes int64,
) error {
	if options == nil || options.HostConfig == nil {
		return fmt.Errorf("docker SDK: missing container configuration")
	}

	if memoryBytes == 0 {
		memoryBytes = defaultReplayMemoryV3
	}

	switch memoryBytes {
	case 256 << 20, 512 << 20, 1 << 30, 2 << 30:
		options.HostConfig.Resources.Memory = memoryBytes
		return nil
	default:
		return fmt.Errorf(
			"docker SDK: unsupported v3 replay memory limit: %d",
			memoryBytes,
		)
	}
}
