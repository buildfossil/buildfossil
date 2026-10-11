package capsule

import "fmt"

// ValidateGoSelectedMetadataCompletenessV3 requires precisely one
// representation for every selected module:
//
//   - a full ZIP/MOD/INFO module; or
//   - an additional metadata-only .info file.
//
// Legacy schema-v3 manifests retain their existing behavior.
func ValidateGoSelectedMetadataCompletenessV3(
	selected []GoModuleIdentityV3,
	full []GoModuleV3,
	infoFiles []GoGraphInfoFileV3,
) error {
	if selected == nil {
		if len(infoFiles) != 0 {
			return fmt.Errorf(
				"capsule: graph info requires an explicit selected graph",
			)
		}
		return nil
	}

	if err := ValidateGoSelectedModulesV3(selected, full); err != nil {
		return err
	}

	if err := ValidateGoGraphInfoFilesV3(
		infoFiles, selected, full,
	); err != nil {
		return err
	}

	fullSet := make(map[string]bool, len(full))
	for _, mod := range full {
		fullSet[mod.Path] = true
	}

	infoSet := make(map[string]bool, len(infoFiles))
	for _, file := range infoFiles {
		infoSet[file.Path] = true
	}

	for _, mod := range selected {
		if !fullSet[mod.Path] && !infoSet[mod.Path] {
			return fmt.Errorf(
				"capsule: selected module lacks ZIP or .info: %s@%s",
				mod.Path, mod.Version,
			)
		}
	}

	return nil
}
