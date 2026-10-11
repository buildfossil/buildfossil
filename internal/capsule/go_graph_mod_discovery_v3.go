package capsule

import (
	"fmt"
	"sort"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
)

// ParseGoGraphModRequirementsV3 identifies module versions referenced
// by go mod graph that are not part of the selected module set.
//
// The input must come from a separately bounded, offline Go command.
func ParseGoGraphModRequirementsV3(
	graph []byte,
	mainModulePath string,
	selected []GoModuleIdentityV3,
) ([]GoModuleIdentityV3, error) {
	const maxGraphBytes = 1 << 20

	if len(graph) > maxGraphBytes {
		return nil, fmt.Errorf("capsule: Go module graph too large")
	}

	if err := module.CheckPath(mainModulePath); err != nil {
		return nil, fmt.Errorf("capsule: invalid main module path: %w", err)
	}

	selectedVersions := make(map[string]bool, len(selected))

	for _, identity := range selected {
		if err := module.Check(identity.Path, identity.Version); err != nil {
			return nil, fmt.Errorf("capsule: invalid selected module: %w", err)
		}

		selectedVersions[identity.Path+"@"+identity.Version] = true
	}

	extras := make(map[string]GoModuleIdentityV3)

	for _, line := range strings.Split(string(graph), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) != 2 {
			return nil, fmt.Errorf("capsule: invalid Go module graph line")
		}

		for index, token := range parts {
			if index == 0 && token == mainModulePath {
				continue
			}

			if strings.HasPrefix(token, "go@") {
				version := strings.TrimPrefix(token, "go@")
				if !modfile.GoVersionRE.MatchString(version) {
					return nil, fmt.Errorf("capsule: invalid Go version graph token")
				}
				continue
			}

			if strings.HasPrefix(token, "toolchain@") {
				version := strings.TrimPrefix(token, "toolchain@")
				if index != 1 ||
					!strings.HasPrefix(version, "go") ||
					!modfile.GoVersionRE.MatchString(strings.TrimPrefix(version, "go")) {
					return nil, fmt.Errorf("capsule: invalid toolchain graph token")
				}
				continue
			}

			path, version, ok := strings.Cut(token, "@")
			if !ok {
				return nil, fmt.Errorf("capsule: invalid Go module graph identity")
			}

			if err := module.Check(path, version); err != nil {
				return nil, fmt.Errorf("capsule: invalid graph module: %w", err)
			}

			if selectedVersions[token] {
				continue
			}

			extras[token] = GoModuleIdentityV3{
				Path:    path,
				Version: version,
			}
		}
	}

	if len(extras) > MaxGoGraphModFilesV3 {
		return nil, fmt.Errorf("capsule: too many additional graph versions")
	}

	result := make([]GoModuleIdentityV3, 0, len(extras))
	for _, identity := range extras {
		result = append(result, identity)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Path == result[j].Path {
			return result[i].Version < result[j].Version
		}
		return result[i].Path < result[j].Path
	})

	return result, nil
}
