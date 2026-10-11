package capsule

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"golang.org/x/mod/module"
)

// GoModuleIdentityV3 represents one selected external module.
type GoModuleIdentityV3 struct {
	Path    string
	Version string
}

type goListModuleV3 struct {
	Path      string          `json:"Path"`
	Version   string          `json:"Version"`
	Main      bool            `json:"Main"`
	Indirect  bool            `json:"Indirect"`
	Dir       string          `json:"Dir"`
	GoMod     string          `json:"GoMod"`
	GoVersion string          `json:"GoVersion"`
	Time      string          `json:"Time"`
	Sum       string          `json:"Sum"`
	GoModSum  string          `json:"GoModSum"`
	Retracted json.RawMessage `json:"Retracted"`
	Replace   json.RawMessage `json:"Replace"`
	Error     json.RawMessage `json:"Error"`
}

// ParseGoModuleGraphV3 parses the JSON stream produced by:
// go list -m -json all
//
// It rejects replacements, invalid identities, duplicate paths,
// missing main modules, and oversized graphs.
func ParseGoModuleGraphV3(data []byte) ([]GoModuleIdentityV3, error) {
	if len(data) == 0 || len(data) > 1<<20 {
		return nil, fmt.Errorf("capsule: invalid Go module graph size")
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	var modules []GoModuleIdentityV3
	seen := make(map[string]bool)
	mainCount := 0

	for {
		var item goListModuleV3

		err := decoder.Decode(&item)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("capsule: decode Go module graph: %w", err)
		}

		if len(item.Replace) != 0 && string(item.Replace) != "null" {
			return nil, fmt.Errorf("capsule: module replacements unsupported")
		}

		if len(item.Error) != 0 && string(item.Error) != "null" {
			return nil, fmt.Errorf("capsule: module graph contains an error")
		}

		if item.Main {
			mainCount++
			if item.Path == "" || item.Version != "" {
				return nil, fmt.Errorf("capsule: invalid main module")
			}
			continue
		}

		if err := module.Check(item.Path, item.Version); err != nil {
			return nil, fmt.Errorf("capsule: invalid module identity: %w", err)
		}

		if seen[item.Path] {
			return nil, fmt.Errorf("capsule: duplicate module path %q", item.Path)
		}
		seen[item.Path] = true

		modules = append(modules, GoModuleIdentityV3{
			Path:    item.Path,
			Version: item.Version,
		})

		if len(modules) > MaxGoSelectedModulesV3 {
			return nil, fmt.Errorf("capsule: too many selected Go modules")
		}
	}

	if mainCount != 1 {
		return nil, fmt.Errorf("capsule: expected exactly one main module")
	}

	sort.Slice(modules, func(i, j int) bool {
		return modules[i].Path < modules[j].Path
	})

	return modules, nil
}
