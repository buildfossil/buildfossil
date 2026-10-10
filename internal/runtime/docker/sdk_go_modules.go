package docker

import (
	"fmt"
	"os"

	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/client"
)

func configureGoModuleArtifacts(
	options *client.ContainerCreateOptions,
	runtime *RuntimeSpec,
	artifactsDir string,
) error {
	if runtime == nil ||
		runtime.Kind != "go" ||
		runtime.Version != "1.26.6" {
		return fmt.Errorf("docker SDK: module artifacts require supported Go runtime")
	}

	if artifactsDir == "" {
		return fmt.Errorf("docker SDK: empty module artifacts directory")
	}

	info, err := os.Lstat(artifactsDir)
	if err != nil {
		return fmt.Errorf("docker SDK: stat module artifacts: %w", err)
	}

	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("docker SDK: invalid module artifacts directory")
	}

	options.HostConfig.Mounts = append(
		options.HostConfig.Mounts,
		mount.Mount{
			Type:     mount.TypeBind,
			Source:   artifactsDir,
			Target:   "/module-artifacts",
			ReadOnly: true,
		},
	)

	if options.HostConfig.Tmpfs == nil {
		options.HostConfig.Tmpfs = make(map[string]string)
	}

	options.HostConfig.Tmpfs["/gomodcache"] =
		"rw,nosuid,nodev,size=64m,mode=1777"

	options.Config.Env = append(
		options.Config.Env,
		"GOMODCACHE=/gomodcache",
	)

	return nil
}
