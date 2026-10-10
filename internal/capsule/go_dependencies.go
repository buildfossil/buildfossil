package capsule

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/mod/modfile"
)

// InspectGoModuleRequirements reports whether go.mod declares module requirements.
// It does not resolve dependencies or contact a registry.
func InspectGoModuleRequirements(workspace string) (bool, error) {
	data, err := os.ReadFile(filepath.Join(workspace, "go.mod"))
	if err != nil {
		return false, fmt.Errorf("read go.mod: %w", err)
	}

	mod, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		return false, fmt.Errorf("parse go.mod: %w", err)
	}

	return len(mod.Require) > 0, nil
}
