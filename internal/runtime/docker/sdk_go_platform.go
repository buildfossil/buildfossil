package docker

import (
	"fmt"

	"github.com/moby/moby/client"
)

func configureGoTargetPlatform(
	options *client.ContainerCreateOptions,
	runtime *RuntimeSpec,
	targetOS string,
	targetArch string,
) error {
	if runtime == nil {
		return fmt.Errorf("docker SDK: Go target requires runtime")
	}

	if runtime.Kind != "go" || runtime.Version != "1.26.6" {
		return fmt.Errorf("docker SDK: unsupported Go runtime")
	}

	switch {
	case targetOS == "linux" && targetArch == "amd64":
	case targetOS == "darwin" && targetArch == "arm64":
	default:
		return fmt.Errorf(
			"docker SDK: unsupported Go target %s/%s",
			targetOS,
			targetArch,
		)
	}

	options.Config.Env = append(
		options.Config.Env,
		"GOOS="+targetOS,
		"GOARCH="+targetArch,
	)

	return nil
}
