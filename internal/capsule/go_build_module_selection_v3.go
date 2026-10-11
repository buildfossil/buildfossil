package capsule

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"golang.org/x/mod/module"
)

type goBuildPackageV3 struct {
	Standard   bool `json:"Standard"`
	Incomplete bool `json:"Incomplete"`

	Module *struct {
		Path    string `json:"Path"`
		Version string `json:"Version"`
		Main    bool   `json:"Main"`
	} `json:"Module"`

	Error *struct {
		Err string `json:"Err"`
	} `json:"Error"`

	DepsErrors []struct {
		Err string `json:"Err"`
	} `json:"DepsErrors"`
}

// SelectGoBuildModulesV3 extracts the external modules used by
// packages in a complete go list -deps -e -json ./... result.
//
// The complete selected module graph is provided separately.
// Dependency loading errors are never treated as an empty
// or successful dependency selection.
func SelectGoBuildModulesV3(
	r io.Reader,
	selected []GoModuleIdentityV3,
) ([]GoModuleIdentityV3, error) {
	limited := &io.LimitedReader{
		R: r,
		N: MaxGoDependencyDiagnosticsSize + 1,
	}

	expected := make(map[string]string, len(selected))

	for _, mod := range selected {
		if err := module.Check(mod.Path, mod.Version); err != nil {
			return nil, fmt.Errorf(
				"capsule: invalid selected module: %w", err,
			)
		}
		if _, exists := expected[mod.Path]; exists {
			return nil, fmt.Errorf(
				"capsule: duplicate selected module %s", mod.Path,
			)
		}
		expected[mod.Path] = mod.Version
	}

	decoder := json.NewDecoder(limited)
	foundPackage := false
	buildModules := make(map[string]string)

	for {
		var pkg goBuildPackageV3
		err := decoder.Decode(&pkg)

		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf(
				"capsule: decode build package: %w", err,
			)
		}

		foundPackage = true

		if pkg.Incomplete ||
			(pkg.Error != nil &&
				strings.TrimSpace(pkg.Error.Err) != "") {
			return nil, fmt.Errorf(
				"capsule: incomplete Go package dependency graph",
			)
		}

		for _, depErr := range pkg.DepsErrors {
			if strings.TrimSpace(depErr.Err) != "" {
				return nil, fmt.Errorf(
					"capsule: Go package dependency resolution failed",
				)
			}
		}

		if pkg.Standard {
			if pkg.Module != nil {
				return nil, fmt.Errorf(
					"capsule: standard library package declares a module",
				)
			}
			continue
		}

		if pkg.Module == nil {
			return nil, fmt.Errorf(
				"capsule: non-standard package missing module metadata",
			)
		}

		if pkg.Module.Main {
			if pkg.Module.Path == "" ||
				pkg.Module.Version != "" {
				return nil, fmt.Errorf(
					"capsule: invalid main package module",
				)
			}
			continue
		}

		if err := module.Check(
			pkg.Module.Path,
			pkg.Module.Version,
		); err != nil {
			return nil, fmt.Errorf(
				"capsule: invalid build module: %w", err,
			)
		}

		version, exists := expected[pkg.Module.Path]
		if !exists || version != pkg.Module.Version {
			return nil, fmt.Errorf(
				"capsule: build module not in selected graph: %s@%s",
				pkg.Module.Path, pkg.Module.Version,
			)
		}

		if previous, exists := buildModules[pkg.Module.Path]; exists &&
			previous != pkg.Module.Version {
			return nil, fmt.Errorf(
				"capsule: conflicting build module versions",
			)
		}

		buildModules[pkg.Module.Path] = pkg.Module.Version
	}

	if limited.N == 0 {
		return nil, fmt.Errorf(
			"capsule: Go package listing exceeds size limit",
		)
	}
	if !foundPackage {
		return nil, fmt.Errorf(
			"capsule: Go package listing contains no packages",
		)
	}

	result := make([]GoModuleIdentityV3, 0, len(buildModules))
	for path, version := range buildModules {
		result = append(result, GoModuleIdentityV3{
			Path: path, Version: version,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Path < result[j].Path
	})

	return result, nil
}
