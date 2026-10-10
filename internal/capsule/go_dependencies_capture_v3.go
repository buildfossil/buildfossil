package capsule

import (
	"fmt"

	"golang.org/x/mod/modfile"
)

// ReadGoDependenciesForCaptureV3 captures zero or one direct Go
// dependency. It never downloads modules.
func ReadGoDependenciesForCaptureV3(
	goMod []byte,
	goSum []byte,
	moduleCacheRoot string,
) ([]GoModuleV3, map[string][]byte, error) {
	parsed, err := modfile.Parse("go.mod", goMod, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("capsule: parse go.mod: %w", err)
	}

	if len(parsed.Replace) != 0 || len(parsed.Exclude) != 0 {
		return nil, nil, fmt.Errorf(
			"capsule: replace/exclude directives are unsupported in v3",
		)
	}

	switch len(parsed.Require) {
	case 0:
		return nil, nil, nil

	case 1:
		if parsed.Require[0].Indirect {
			return nil, nil, fmt.Errorf(
				"capsule: v3 capture requires a direct Go dependency",
			)
		}

		mod, artifacts, err := ReadGoModuleArtifactsForCaptureV3(
			goMod,
			goSum,
			moduleCacheRoot,
		)
		if err != nil {
			return nil, nil, err
		}

		return []GoModuleV3{mod}, artifacts, nil

	default:
		return nil, nil, fmt.Errorf(
			"capsule: v3 capture supports at most one Go dependency, found %d",
			len(parsed.Require),
		)
	}
}
