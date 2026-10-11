package capsule

import (
	"fmt"

	"golang.org/x/mod/modfile"
)

// ValidateGoSelectedModuleConsistencyV3 checks root go.mod
// requirements against the effective selected module build list.
//
// Legacy schema-v3 manifests derive that list from full ZIP modules.
// Extended manifests use GoSelectedModules.
func ValidateGoSelectedModuleConsistencyV3(
	goMod []byte,
	manifest Manifest,
) error {
	if manifest.GoSelectedModules == nil {
		return ValidateGoModuleConsistencyV3(
			goMod, manifest.GoModules,
		)
	}

	if err := ValidateGoSelectedModulesV3(
		manifest.GoSelectedModules,
		manifest.GoModules,
	); err != nil {
		return err
	}

	parsed, err := modfile.Parse("go.mod", goMod, nil)
	if err != nil {
		return fmt.Errorf(
			"capsule: parse workspace go.mod: %w", err,
		)
	}

	if parsed.Module == nil {
		return fmt.Errorf(
			"capsule: missing root Go module declaration",
		)
	}

	if len(parsed.Replace) != 0 || len(parsed.Exclude) != 0 {
		return fmt.Errorf(
			"capsule: unsupported replace/exclude in workspace go.mod",
		)
	}

	selected := make(
		map[string]string,
		len(manifest.GoSelectedModules),
	)

	for _, mod := range manifest.GoSelectedModules {
		selected[mod.Path] = mod.Version
	}

	if len(parsed.Require) == 0 && len(selected) != 0 {
		return fmt.Errorf(
			"capsule: unexpected selected modules for dependency-free workspace",
		)
	}

	for _, required := range parsed.Require {
		version, exists := selected[required.Mod.Path]

		if !exists {
			return fmt.Errorf(
				"capsule: required module missing from selected graph: %s",
				required.Mod.Path,
			)
		}

		if version != required.Mod.Version {
			return fmt.Errorf(
				"capsule: required module version mismatch: %s",
				required.Mod.Path,
			)
		}
	}

	return nil
}
