package capsule

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// goSourcePackageV3 contains only the go list fields needed to select
// build inputs from the main Go module.
type goSourcePackageV3 struct {
	Dir        string   `json:"Dir"`
	GoFiles    []string `json:"GoFiles"`
	Incomplete bool     `json:"Incomplete"`

	Error *struct {
		Err string `json:"Err"`
	} `json:"Error"`

	Module *struct {
		Main bool   `json:"Main"`
		Dir  string `json:"Dir"`
	} `json:"Module"`
}

// ParseGoSourceFilesV3 selects Go source files from bounded
// go list -e -json ./... metadata. It does not read source contents.
//
// GoFiles reflects the selected platform and build constraints.
// Incomplete package metadata is not automatically fatal because
// build failures can still have a valid GoFiles selection.
func ParseGoSourceFilesV3(
	r io.Reader,
	workspaceRoot string,
) ([]string, error) {
	if r == nil || workspaceRoot == "" {
		return nil, fmt.Errorf("capsule: missing Go source discovery input")
	}

	root, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return nil, fmt.Errorf("capsule: resolve workspace root: %w", err)
	}

	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("capsule: canonicalize workspace root: %w", err)
	}

	rootInfo, err := os.Stat(root)
	if err != nil || !rootInfo.IsDir() {
		return nil, fmt.Errorf("capsule: invalid workspace root")
	}

	limited := &io.LimitedReader{
		R: r,
		N: MaxGoDependencyDiagnosticsSize + 1,
	}
	decoder := json.NewDecoder(limited)

	foundPackage := false
	foundMainModule := false
	paths := make(map[string]struct{})

	for {
		var pkg goSourcePackageV3

		err := decoder.Decode(&pkg)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf(
				"capsule: decode Go source metadata: %w", err,
			)
		}

		foundPackage = true

		if pkg.Module == nil || !pkg.Module.Main {
			continue
		}

		foundMainModule = true

		// An incomplete main-module package without selected GoFiles
		// cannot be treated as a successful partial discovery.
		// Dependency errors remain acceptable when GoFiles are known.
		if pkg.Incomplete && len(pkg.GoFiles) == 0 {
			return nil, fmt.Errorf(
				"capsule: incomplete Go source selection for package %q",
				pkg.Dir,
			)
		}

		if pkg.Error != nil && pkg.Error.Err != "" {
			return nil, fmt.Errorf(
				"capsule: Go source package discovery failed: %s",
				pkg.Error.Err,
			)
		}

		moduleDir := filepath.Clean(pkg.Module.Dir)
		moduleInfo, err := os.Stat(moduleDir)
		if err != nil {
			return nil, fmt.Errorf(
				"capsule: stat main module directory: %w", err,
			)
		}

		if !os.SameFile(moduleInfo, rootInfo) {
			return nil, fmt.Errorf(
				"capsule: unexpected main module directory",
			)
		}

		dir := filepath.Clean(pkg.Dir)
		packageRel, err := filepath.Rel(root, dir)

		if err != nil || packageRel == ".." ||
			strings.HasPrefix(
				packageRel, ".."+string(filepath.Separator),
			) {
			packageRel, err = filepath.Rel(moduleDir, dir)
		}

		if err != nil || packageRel == ".." ||
			strings.HasPrefix(
				packageRel, ".."+string(filepath.Separator),
			) {
			return nil, fmt.Errorf(
				"capsule: Go package outside workspace",
			)
		}

		// Check package-directory components without following
		// symlinks. Workspace reading performs its own checks later.
		current := root

		if packageRel != "." {
			for _, component := range strings.Split(
				packageRel, string(filepath.Separator),
			) {
				current = filepath.Join(current, component)

				info, err := os.Lstat(current)
				if err != nil {
					return nil, fmt.Errorf(
						"capsule: inspect Go package directory: %w",
						err,
					)
				}

				if !info.IsDir() ||
					info.Mode()&os.ModeSymlink != 0 {
					return nil, fmt.Errorf(
						"capsule: invalid Go package directory: %q",
						current,
					)
				}
			}
		}

		for _, source := range pkg.GoFiles {
			if err := ValidateWorkspacePath(source, 0600); err != nil {
				return nil, fmt.Errorf(
					"capsule: invalid Go source path %q: %w",
					source, err,
				)
			}

			rel := filepath.ToSlash(filepath.Join(
				packageRel,
				filepath.FromSlash(source),
			))

			if err := ValidateWorkspacePath(rel, 0600); err != nil {
				return nil, fmt.Errorf(
					"capsule: invalid workspace source path %q: %w",
					rel, err,
				)
			}

			if _, exists := paths[rel]; !exists &&
				len(paths) >= MaxWorkspaceV3Files {
				return nil, fmt.Errorf(
					"capsule: too many discovered Go source files",
				)
			}

			paths[rel] = struct{}{}
		}
	}

	if limited.N == 0 {
		return nil, fmt.Errorf(
			"capsule: Go source metadata exceeds output limit",
		)
	}

	if !foundPackage {
		return nil, fmt.Errorf("capsule: no Go package metadata")
	}

	if !foundMainModule {
		return nil, fmt.Errorf(
			"capsule: no main-module Go packages found",
		)
	}

	if len(paths) == 0 {
		return nil, fmt.Errorf(
			"capsule: no main-module Go source files found",
		)
	}

	result := make([]string, 0, len(paths))
	for name := range paths {
		result = append(result, name)
	}
	sort.Strings(result)

	return result, nil
}
