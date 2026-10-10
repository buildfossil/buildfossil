package replay

import (
	"fmt"

	"github.com/buildfossil/buildfossil/internal/capsule"
)

func validatePortableGoReplay(manifest capsule.Manifest) error {
	if manifest.Runtime == nil ||
		manifest.Runtime.Kind != "go" ||
		manifest.Runtime.Version != capsule.SupportedGoVersion {
		return fmt.Errorf("replay: portable Go requires supported Go runtime")
	}

	if manifest.GoBuildEnv == nil {
		return fmt.Errorf("replay: portable Go requires recorded build environment")
	}

	if manifest.Platform.OS != "darwin" ||
		manifest.Platform.Architecture != "arm64" {
		return fmt.Errorf("replay: unsupported portable Go capture platform")
	}

	env := manifest.GoBuildEnv

	if env.CGOEnabled != "0" {
		return fmt.Errorf("replay: portable Go requires CGO_ENABLED=0")
	}

	if env.GOFLAGS != "" {
		return fmt.Errorf("replay: portable Go requires empty GOFLAGS")
	}

	switch {
	case env.GOOS == "darwin" && env.GOARCH == "arm64":
		// Preserve the existing native macOS Go replay policy.
		return env.ValidatePortableGoBuild()

	case env.GOOS == "linux" && env.GOARCH == "amd64":
		// Explicitly supported cross-build:
		// macOS ARM64 capture -> Linux AMD64 Go build.
		return nil

	default:
		return fmt.Errorf(
			"replay: unsupported Go build target %s/%s",
			env.GOOS,
			env.GOARCH,
		)
	}
}
