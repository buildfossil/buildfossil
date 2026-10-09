package docker

import (
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/client"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func newSecureContainerOptions(
	workspace string,
	argv []string,
	user string,
) client.ContainerCreateOptions {
	pidsLimit := int64(64)

	return client.ContainerCreateOptions{
		Config: &container.Config{
			Image:        image,
			User:         user,
			Cmd:          argv,
			WorkingDir:   "/workspace",
			Tty:          false,
			AttachStdout: true,
			AttachStderr: true,
		},
		HostConfig: &container.HostConfig{
			NetworkMode:    "none",
			CapDrop:        []string{"ALL"},
			SecurityOpt:    []string{"no-new-privileges"},
			ReadonlyRootfs: true,
			Resources: container.Resources{
				PidsLimit: &pidsLimit,
				Memory:    256 * 1024 * 1024,
				NanoCPUs:  1_000_000_000,
			},
			Mounts: []mount.Mount{
				{
					Type:     mount.TypeBind,
					Source:   workspace,
					Target:   "/workspace",
					ReadOnly: true,
				},
			},
		},
		Platform: &ocispec.Platform{
			OS:           "linux",
			Architecture: "amd64",
		},
	}
}
