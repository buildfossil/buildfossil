package capsule

import (
	"fmt"

	"golang.org/x/mod/modfile"
)

// ValidateGoModuleConsistencyV3 checks that the captured go.mod
// agrees with the dependencies declared in the v3 manifest.
func ValidateGoModuleConsistencyV3(
	goMod []byte,
	modules []GoModuleV3,
) error {
	parsed, err := modfile.Parse("go.mod", goMod, nil)
	if err != nil {
		return fmt.Errorf("capsule: parse workspace go.mod: %w", err)
	}

	if parsed.Module == nil {
		return fmt.Errorf("capsule: missing module declaration in go.mod")
	}

	if len(parsed.Replace) != 0 || len(parsed.Exclude) != 0 {
		return fmt.Errorf(
			"capsule: unsupported replace/exclude in workspace go.mod",
		)
	}

	if len(parsed.Require) > MaxGoModulesV3 {
		return fmt.Errorf("capsule: unsupported Go dependency count")
	}

	if err := ValidateGoModulesV3(modules); err != nil {
		return err
	}

	if len(parsed.Require) != len(modules) {
		return fmt.Errorf(
			"capsule: go.mod dependencies do not match manifest",
		)
	}

	if len(parsed.Require) == 0 {
		return nil
	}

	required := parsed.Require[0]
	if required.Indirect {
		return fmt.Errorf(
			"capsule: indirect-only dependency is unsupported",
		)
	}

	if required.Mod.Path != modules[0].Path ||
		required.Mod.Version != modules[0].Version {
		return fmt.Errorf(
			"capsule: go.mod module identity does not match manifest",
		)
	}

	return nil
}
