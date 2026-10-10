package docker

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/client"
)

const snapshotCleanupTimeout = 15 * time.Second

// SnapshotVolumes owns Docker resources created for one replay.
// Close must be called before closing the Docker client.
//
// Not safe for concurrent use.
type SnapshotVolumes struct {
	client      *client.Client
	Workspace   string
	ModuleCache string
	containers  []string
}

func newSnapshotVolumeName() (string, error) {
	var random [16]byte

	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("docker SDK: generate volume ID: %w", err)
	}

	return "buildfossil-snapshot-" +
		hex.EncodeToString(random[:]), nil
}

// CreateSnapshotVolumes uploads two in-memory TAR archives
// into separate Docker-managed volumes.
//
// image must already be available to the Docker daemon.
// The caller owns the returned volumes and must call Close.
func CreateSnapshotVolumes(
	ctx context.Context,
	dockerClient *client.Client,
	image string,
	workspaceTAR, moduleTAR []byte,
) (result *SnapshotVolumes, err error) {
	if dockerClient == nil {
		return nil, fmt.Errorf("docker SDK: nil client")
	}
	if image == "" {
		return nil, fmt.Errorf("docker SDK: empty snapshot image")
	}
	if len(workspaceTAR) == 0 || len(moduleTAR) == 0 {
		return nil, fmt.Errorf("docker SDK: empty snapshot archive")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	volumes := &SnapshotVolumes{
		client: dockerClient,
	}

	// Roll back all created resources if any step fails.
	defer func() {
		if err != nil {
			err = errors.Join(err, volumes.Close())
		}
	}()

	for _, snapshot := range []struct {
		kind string
		data []byte
	}{
		{"workspace", workspaceTAR},
		{"module-cache", moduleTAR},
	} {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		name, err := newSnapshotVolumeName()
		if err != nil {
			return nil, err
		}

		// Record the generated name before contacting Docker.
		// The API request may succeed even if its response is lost.
		if snapshot.kind == "workspace" {
			volumes.Workspace = name
		} else {
			volumes.ModuleCache = name
		}

		_, err = dockerClient.VolumeCreate(
			ctx,
			client.VolumeCreateOptions{
				Name: name,
				Labels: map[string]string{
					"org.buildfossil.component": "snapshot",
					"org.buildfossil.kind":      snapshot.kind,
				},
			},
		)
		if err != nil {
			return nil, fmt.Errorf(
				"docker SDK: create %s volume: %w",
				snapshot.kind, err,
			)
		}

		// Use a temporary container solely as a Docker API
		// destination for copying the TAR into the volume.
		options := newSecureContainerOptions(
			"",
			[]string{"/bin/sh", "-c", "sleep 60"},
			"10001:10001",
		)

		options.Config.Image = image
		options.Config.WorkingDir = "/"

		// Do not inherit the workspace bind mount.
		options.HostConfig.Mounts = []mount.Mount{
			{
				Type:   mount.TypeVolume,
				Source: name,
				Target: "/snapshot",
				VolumeOptions: &mount.VolumeOptions{
					NoCopy: true,
				},
			},
		}

		created, err := dockerClient.ContainerCreate(ctx, options)
		if err != nil && !errdefs.IsNotFound(err) {
			return nil, fmt.Errorf(
				"docker SDK: create %s preparation container: %w",
				snapshot.kind, err,
			)
		}

		volumes.containers = append(
			volumes.containers,
			created.ID,
		)

		_, err = dockerClient.CopyToContainer(
			ctx,
			created.ID,
			client.CopyToContainerOptions{
				DestinationPath: "/snapshot",
				Content:         bytes.NewReader(snapshot.data),
			},
		)
		if err != nil {
			return nil, fmt.Errorf(
				"docker SDK: upload %s snapshot: %w",
				snapshot.kind, err,
			)
		}
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Preparation containers are no longer needed.
	if err := volumes.removePreparationContainers(); err != nil {
		return nil, err
	}

	return volumes, nil
}

// removePreparationContainers removes temporary upload containers.
func (v *SnapshotVolumes) removePreparationContainers() error {
	if v == nil || v.client == nil {
		return nil
	}

	var result error
	remaining := make([]string, 0)

	for i := len(v.containers) - 1; i >= 0; i-- {
		id := v.containers[i]

		cleanupCtx, cancel := context.WithTimeout(
			context.Background(),
			snapshotCleanupTimeout,
		)

		_, err := v.client.ContainerRemove(
			cleanupCtx,
			id,
			client.ContainerRemoveOptions{Force: true},
		)
		cancel()

		if err != nil {
			result = errors.Join(
				result,
				fmt.Errorf(
					"docker SDK: remove preparation container %s: %w",
					id, err,
				),
			)
			remaining = append(remaining, id)
		}
	}

	v.containers = remaining
	return result
}

// Close removes the volumes and any remaining preparation
// containers. It uses a fresh context even if replay was canceled.
func (v *SnapshotVolumes) Close() error {
	if v == nil || v.client == nil {
		return nil
	}

	result := v.removePreparationContainers()

	for _, name := range []string{
		v.ModuleCache,
		v.Workspace,
	} {
		if name == "" {
			continue
		}

		cleanupCtx, cancel := context.WithTimeout(
			context.Background(),
			snapshotCleanupTimeout,
		)

		_, err := v.client.VolumeRemove(
			cleanupCtx,
			name,
			client.VolumeRemoveOptions{},
		)
		cancel()

		if err != nil {
			result = errors.Join(
				result,
				fmt.Errorf(
					"docker SDK: remove snapshot volume %s: %w",
					name, err,
				),
			)
			continue
		}

		if name == v.Workspace {
			v.Workspace = ""
		}
		if name == v.ModuleCache {
			v.ModuleCache = ""
		}
	}

	return result
}
