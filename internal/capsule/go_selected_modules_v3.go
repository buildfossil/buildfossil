package capsule

import (
	"fmt"

	"golang.org/x/mod/module"
)

const MaxGoSelectedModulesV3 = 128

// ValidateGoSelectedModulesV3 checks a complete, strictly ordered
// selected Go module build list, independently of ZIP availability.
//
// An absent list means legacy schema-v3 behavior. An explicit list
// must include every module represented by a full ZIP artifact.
func ValidateGoSelectedModulesV3(
	selected []GoModuleIdentityV3,
	full []GoModuleV3,
) error {
	if selected == nil {
		return nil
	}

	if len(selected) > MaxGoSelectedModulesV3 {
		return fmt.Errorf("capsule: too many selected Go modules")
	}

	versions := make(map[string]string, len(selected))
	previous := ""

	for index, mod := range selected {
		if err := module.Check(mod.Path, mod.Version); err != nil {
			return fmt.Errorf(
				"capsule: invalid selected Go module: %w", err,
			)
		}

		if index > 0 && mod.Path <= previous {
			return fmt.Errorf(
				"capsule: selected Go modules must have unique sorted paths",
			)
		}

		previous = mod.Path
		versions[mod.Path] = mod.Version
	}

	for _, mod := range full {
		version, exists := versions[mod.Path]
		if !exists || version != mod.Version {
			return fmt.Errorf(
				"capsule: full Go module missing or mismatched in selected graph: %s@%s",
				mod.Path, mod.Version,
			)
		}
	}

	return nil
}

// EffectiveGoSelectedModulesV3 returns the explicit selected graph,
// or the complete ZIP-module list used by legacy schema-v3 capsules.
func EffectiveGoSelectedModulesV3(
	manifest Manifest,
) []GoModuleIdentityV3 {
	if manifest.GoSelectedModules != nil {
		return manifest.GoSelectedModules
	}

	result := make(
		[]GoModuleIdentityV3, 0, len(manifest.GoModules),
	)

	for _, mod := range manifest.GoModules {
		result = append(result, GoModuleIdentityV3{
			Path:    mod.Path,
			Version: mod.Version,
		})
	}

	return result
}
