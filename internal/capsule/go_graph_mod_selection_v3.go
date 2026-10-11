package capsule

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/mod/module"
)

var goSumHashPatternV3 = regexp.MustCompile(`^h1:[A-Za-z0-9+/]{43}=$`)

// SelectVerifiedGoGraphModsV3 selects additional .mod versions
// whose /go.mod checksums are declared in the captured go.sum.
//
// Full ZIP modules already contain a verified .mod. Metadata-only
// selected modules and historical graph versions may need .mod.
func SelectVerifiedGoGraphModsV3(
	selected []GoModuleIdentityV3,
	full []GoModuleV3,
	historical []GoModuleIdentityV3,
	goSum []byte,
) ([]GoModuleIdentityV3, error) {
	fullSet := make(map[string]bool, len(full))

	for _, mod := range full {
		if err := module.Check(mod.Path, mod.Version); err != nil {
			return nil, fmt.Errorf(
				"capsule: invalid full Go module: %w", err,
			)
		}
		fullSet[mod.Path+"@"+mod.Version] = true
	}

	checksums := make(map[string]bool)
	scanner := bufio.NewScanner(bytes.NewReader(goSum))
	scanner.Buffer(make([]byte, 4096), 1<<20)

	for scanner.Scan() {
		parts := strings.Fields(scanner.Text())
		if len(parts) != 3 {
			return nil, fmt.Errorf("capsule: invalid go.sum line")
		}

		version := strings.TrimSuffix(parts[1], "/go.mod")
		if err := module.Check(parts[0], version); err != nil {
			return nil, fmt.Errorf(
				"capsule: invalid go.sum module identity: %w", err,
			)
		}

		if !goSumHashPatternV3.MatchString(parts[2]) {
			return nil, fmt.Errorf("capsule: invalid go.sum checksum")
		}

		if strings.HasSuffix(parts[1], "/go.mod") {
			identity := parts[0] + "@" +
				strings.TrimSuffix(parts[1], "/go.mod")
			checksums[identity] = true
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf(
			"capsule: read go.sum: %w", err,
		)
	}

	candidates := make(map[string]GoModuleIdentityV3)

	for _, mod := range append(
		append([]GoModuleIdentityV3(nil), selected...),
		historical...,
	) {
		if err := module.Check(mod.Path, mod.Version); err != nil {
			return nil, fmt.Errorf(
				"capsule: invalid graph module: %w", err,
			)
		}

		identity := mod.Path + "@" + mod.Version

		if fullSet[identity] || !checksums[identity] {
			continue
		}

		candidates[identity] = mod
	}

	result := make(
		[]GoModuleIdentityV3, 0, len(candidates),
	)
	for _, mod := range candidates {
		result = append(result, mod)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Path == result[j].Path {
			return result[i].Version < result[j].Version
		}
		return result[i].Path < result[j].Path
	})

	if len(result) > MaxGoGraphModFilesV3 {
		return nil, fmt.Errorf(
			"capsule: too many verified graph mod files",
		)
	}

	return result, nil
}
