package replay

import (
	"fmt"

	"github.com/buildfossil/buildfossil/internal/capsule"
)

func validateReplayPlatform(manifest capsule.Manifest) error {
	platform := manifest.Platform

	if platform.OS == "linux" && platform.Architecture == "amd64" {
		return nil
	}

	if platform.OS == "darwin" && platform.Architecture == "arm64" {
		return validatePortableGoReplay(manifest)
	}

	return fmt.Errorf(
		"replay: unsupported source platform %s/%s; expected linux/amd64",
		platform.OS,
		platform.Architecture,
	)
}
