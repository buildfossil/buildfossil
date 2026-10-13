package docker

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
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
	client       *client.Client
	Workspace    string
	ModuleCache  string
	containers   []string
	volumeTokens map[string]string
	volumeKinds  map[string]string
}

func newSnapshotVolumeName() (string, error) {
	var random [16]byte

	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("docker SDK: generate volume ID: %w", err)
	}

	return "rew-snapshot-" +
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
) (*SnapshotVolumes, error) {
	return createSnapshotVolumesWithNameGenerator(
		ctx, dockerClient, image, workspaceTAR, moduleTAR,
		newSnapshotVolumeName,
	)
}

// CreateSnapshotVolumesFromReaders uploads two TAR streams into
// Docker-managed volumes without materializing them as byte slices.
//
// Readers must remain valid until this call returns. They are consumed
// once and are not closed by this function.
func CreateSnapshotVolumesFromReaders(
	ctx context.Context,
	dockerClient *client.Client,
	image string,
	workspaceTAR, moduleTAR io.Reader,
) (*SnapshotVolumes, error) {
	return createSnapshotVolumesFromReadersWithNameGenerator(
		ctx, dockerClient, image,
		workspaceTAR, moduleTAR,
		newSnapshotVolumeName,
	)
}

// createSnapshotVolumesWithNameGenerator permits deterministic resource
// identification in tests while production uses random volume names.
func createSnapshotVolumesWithNameGenerator(
	ctx context.Context,
	dockerClient *client.Client,
	image string,
	workspaceTAR, moduleTAR []byte,
	generateName func() (string, error),
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
	return createSnapshotVolumesFromReadersWithNameGenerator(
		ctx, dockerClient, image,
		bytes.NewReader(workspaceTAR),
		bytes.NewReader(moduleTAR),
		generateName,
	)
}

func createSnapshotVolumesFromReadersWithNameGenerator(
	ctx context.Context,
	dockerClient *client.Client,
	image string,
	workspaceTAR, moduleTAR io.Reader,
	generateName func() (string, error),
) (result *SnapshotVolumes, err error) {
	if dockerClient == nil {
		return nil, fmt.Errorf("docker SDK: nil client")
	}
	if image == "" {
		return nil, fmt.Errorf("docker SDK: empty snapshot image")
	}
	if workspaceTAR == nil || moduleTAR == nil {
		return nil, fmt.Errorf("docker SDK: nil snapshot reader")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	volumes := &SnapshotVolumes{
		client:       dockerClient,
		volumeTokens: make(map[string]string),
		volumeKinds:  make(map[string]string),
	}

	// Roll back all created resources if any step fails.
	defer func() {
		if err != nil {
			err = errors.Join(err, volumes.Close())
		}
	}()

	for _, snapshot := range []struct {
		kind string
		data io.Reader
	}{
		{"workspace", workspaceTAR},
		{"module-cache", moduleTAR},
	} {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		name, err := generateName()
		if err != nil {
			return nil, err
		}

		token, err := newRunID()
		if err != nil {
			return nil, err
		}

		// Record identity before contacting Docker: creation may succeed
		// even if its HTTP response is lost.
		volumes.volumeTokens[name] = token
		volumes.volumeKinds[name] = snapshot.kind
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
					"org.rew.component":       "snapshot",
					"org.rew.kind":            snapshot.kind,
					"org.rew.ownership-token": token,
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

		ownershipToken, err := newRunID()
		if err != nil {
			return nil, err
		}

		containerName := "rew-snapshot-prep-" + ownershipToken
		options.Name = containerName
		options.Config.Labels["org.rew.ownership-token"] = ownershipToken

		created, err := dockerClient.ContainerCreate(ctx, options)
		if err != nil {
			cleanupErr := cleanupUnknownPreparationContainer(
				dockerClient, containerName, ownershipToken,
			)
			return nil, errors.Join(
				fmt.Errorf(
					"docker SDK: create %s preparation container: %w",
					snapshot.kind, err,
				),
				cleanupErr,
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
				Content:         snapshot.data,
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

		// The container may already have been removed after an
		// ambiguous Docker API response. Its recorded ID is then gone.
		if errdefs.IsNotFound(err) {
			continue
		}
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
	if result != nil {
		// Do not remove snapshot volumes while preparation containers
		// may still reference them. Preserve state for a later Close.
		return result
	}

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

		err := v.removeOwnedVolume(cleanupCtx, name)
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

		delete(v.volumeTokens, name)
		delete(v.volumeKinds, name)
		if name == v.Workspace {
			v.Workspace = ""
		}
		if name == v.ModuleCache {
			v.ModuleCache = ""
		}
	}

	return result
}

// removeOwnedVolume never deletes a volume without verifying ownership.
// This also protects rollback after an ambiguous VolumeCreate response.
func (v *SnapshotVolumes) removeOwnedVolume(
	ctx context.Context,
	name string,
) error {
	token := v.volumeTokens[name]
	kind := v.volumeKinds[name]
	if token == "" || kind == "" {
		return fmt.Errorf("docker SDK: unverified snapshot volume ownership: %s", name)
	}

	inspected, err := v.client.VolumeInspect(
		ctx, name, client.VolumeInspectOptions{},
	)
	if errdefs.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("docker SDK: inspect snapshot volume %s: %w", name, err)
	}

	if inspected.Volume.Name != name ||
		inspected.Volume.Labels["org.rew.component"] != "snapshot" ||
		inspected.Volume.Labels["org.rew.kind"] != kind ||
		inspected.Volume.Labels["org.rew.ownership-token"] != token {
		return fmt.Errorf("docker SDK: unverified snapshot volume ownership: %s", name)
	}

	_, err = v.client.VolumeRemove(
		ctx, name, client.VolumeRemoveOptions{},
	)
	if err != nil {
		return fmt.Errorf("docker SDK: remove owned snapshot volume %s: %w", name, err)
	}
	return nil
}

// cleanupUnknownPreparationContainer handles an ambiguous ContainerCreate result.
// Never remove a container unless its ownership token matches the attempted create.
func cleanupUnknownPreparationContainer(
	dockerClient *client.Client,
	name string,
	token string,
) error {
	return cleanupUnknownContainer(
		dockerClient,
		name,
		token,
		"preparation",
		client.ContainerRemoveOptions{Force: true},
	)
}

func cleanupUnknownContainer(
	dockerClient *client.Client,
	name string,
	token string,
	kind string,
	removeOptions client.ContainerRemoveOptions,
) error {
	ctx, cancel := context.WithTimeout(
		context.Background(), snapshotCleanupTimeout,
	)
	defer cancel()

	inspected, err := dockerClient.ContainerInspect(
		ctx, name, client.ContainerInspectOptions{},
	)
	if errdefs.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf(
			"docker SDK: inspect uncertain %s container %s: %w",
			kind, name, err,
		)
	}

	if inspected.Container.Config == nil ||
		inspected.Container.Name != "/"+name ||
		inspected.Container.Config.Labels["org.rew.component"] != "replay" ||
		inspected.Container.Config.Labels["org.rew.ownership-token"] != token ||
		inspected.Container.ID == "" {
		return fmt.Errorf(
			"docker SDK: uncertain %s container %s has unverified ownership",
			kind, name,
		)
	}

	_, err = dockerClient.ContainerRemove(
		ctx,
		inspected.Container.ID,
		removeOptions,
	)
	if err != nil {
		return fmt.Errorf(
			"docker SDK: remove uncertain %s container %s: %w",
			kind, name, err,
		)
	}
	return nil
}
