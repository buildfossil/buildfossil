package capsule

import (
	"fmt"

	"golang.org/x/mod/modfile"
)

// ValidateGoModuleConsistencyV3 checks that every root go.mod
// requirement exists in the manifest with the same version.
//
// Additional manifest modules may be transitive dependencies.
// This check does not prove that the manifest contains the
// complete selected Go module graph.
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

	if err := ValidateGoModulesV3(modules); err != nil {
		return err
	}

	// A dependency-free root module must not declare
	// unrelated external modules.
	if len(parsed.Require) == 0 && len(modules) != 0 {
		return fmt.Errorf(
			"capsule: unexpected Go modules for dependency-free workspace",
		)
	}

	selected := make(map[string]string, len(modules))
	for _, mod := range modules {
		selected[mod.Path] = mod.Version
	}

	for _, required := range parsed.Require {
		version, ok := selected[required.Mod.Path]
		if !ok {
			return fmt.Errorf(
				"capsule: required Go module missing from manifest: %s",
				required.Mod.Path,
			)
		}

		if version != required.Mod.Version {
			return fmt.Errorf(
				"capsule: Go module version mismatch for %s",
				required.Mod.Path,
			)
		}
	}

	return nil
}
