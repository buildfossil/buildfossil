package docker

import (
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/client"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

const replayImage = "ghcr.io/buildfossil/replay-base@sha256:51ea5ed5cda2f1133ed5b0c07f7f26902e0ed7f9527671a3491525bbc080c944"
const goReplayImage = "docker.io/library/golang@sha256:1a9c10cf505a9e6b1e96ea77ebdbfe79a0f10380181faf88bc3b51d7e4315fae"

func newSecureContainerOptions(
	workspace string,
	argv []string,
	user string,
) client.ContainerCreateOptions {
	pidsLimit := int64(64)

	return client.ContainerCreateOptions{
		Config: &container.Config{
			Image:        replayImage,
			User:         user,
			Cmd:          argv,
			WorkingDir:   "/workspace",
			Tty:          false,
			AttachStdout: true,
			AttachStderr: true,
			Labels: map[string]string{
				"org.buildfossil.component": "replay",
			},
		},
		HostConfig: &container.HostConfig{
			LogConfig: container.LogConfig{
				Type: "json-file",
				Config: map[string]string{
					"max-size": "1m",
					"max-file": "1",
				},
			},
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
