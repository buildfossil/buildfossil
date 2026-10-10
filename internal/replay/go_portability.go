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

	env := manifest.GoBuildEnv

	if env.GOOS != manifest.Platform.OS ||
		env.GOARCH != manifest.Platform.Architecture {
		return fmt.Errorf(
			"replay: Go build target does not match capture platform",
		)
	}

	if err := env.ValidatePortableGoBuild(); err != nil {
		return fmt.Errorf("replay: %w", err)
	}

	return nil
}
