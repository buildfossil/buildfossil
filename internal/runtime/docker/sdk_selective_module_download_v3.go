package docker

import (
	"fmt"

	"golang.org/x/mod/module"
)

// selectiveGoModuleDownloadArgsV3 creates an offline download
// command for modules whose complete ZIP artifacts are available.
//
// It must never request metadata-only selected modules.
func selectiveGoModuleDownloadArgsV3(
	expected []ExpectedGoModule,
	materialized []ExpectedGoModule,
) ([]string, error) {
	selected := make(map[string]string, len(expected))

	for _, mod := range expected {
		if err := module.Check(mod.Path, mod.Version); err != nil {
			return nil, fmt.Errorf(
				"docker SDK: invalid selected module: %w", err,
			)
		}

		if _, exists := selected[mod.Path]; exists {
			return nil, fmt.Errorf(
				"docker SDK: duplicate selected module %s", mod.Path,
			)
		}

		selected[mod.Path] = mod.Version
	}

	args := []string{"go", "mod", "download"}
	previous := ""

	for _, mod := range materialized {
		if err := module.Check(mod.Path, mod.Version); err != nil {
			return nil, fmt.Errorf(
				"docker SDK: invalid materialized module: %w", err,
			)
		}

		if previous != "" && mod.Path <= previous {
			return nil, fmt.Errorf(
				"docker SDK: materialized modules not sorted and unique",
			)
		}
		previous = mod.Path

		version, exists := selected[mod.Path]
		if !exists || version != mod.Version {
			return nil, fmt.Errorf(
				"docker SDK: materialized module not in selected graph",
			)
		}

		args = append(args, mod.Path+"@"+mod.Version)
	}

	if len(materialized) == 0 {
		return nil, nil
	}

	return args, nil
}
