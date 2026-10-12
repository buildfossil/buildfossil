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

type goEmbedPackageV3 struct {
	Dir        string   `json:"Dir"`
	Incomplete bool     `json:"Incomplete"`
	EmbedFiles []string `json:"EmbedFiles"`

	Module *struct {
		Main bool   `json:"Main"`
		Dir  string `json:"Dir"`
	} `json:"Module"`

	Error *struct {
		Err string `json:"Err"`
	} `json:"Error"`

	DepsErrors []struct {
		Err string `json:"Err"`
	} `json:"DepsErrors"`
}

// ParseGoEmbedFilesV3 extracts main-workspace embedded files from
// bounded go list -e -json ./... output. It does not read files.
func ParseGoEmbedFilesV3(
	r io.Reader,
	workspaceRoot string,
) ([]string, error) {
	if r == nil || workspaceRoot == "" {
		return nil, fmt.Errorf("capsule: missing embed discovery input")
	}

	root, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return nil, fmt.Errorf("capsule: resolve workspace root: %w", err)
	}

	// Go may report the physical workspace path, while callers
	// use a system alias (for example, /var on macOS).
	// Canonicalize only the trusted workspace root.
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("capsule: canonicalize workspace root: %w", err)
	}

	limited := &io.LimitedReader{
		R: r,
		N: MaxGoDependencyDiagnosticsSize + 1,
	}
	decoder := json.NewDecoder(limited)

	found := false
	foundMainModule := false
	paths := make(map[string]struct{})

	for {
		var pkg goEmbedPackageV3

		err := decoder.Decode(&pkg)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("capsule: decode Go embed metadata: %w", err)
		}

		found = true

		if pkg.Incomplete ||
			(pkg.Error != nil && pkg.Error.Err != "") ||
			len(pkg.DepsErrors) != 0 {
			return nil, fmt.Errorf("capsule: incomplete Go embed package metadata")
		}

		if pkg.Module == nil || !pkg.Module.Main {
			continue
		}
		foundMainModule = true

		// Go may report /var while the workspace was opened through
		// /private/var on macOS. Establish root identity without
		// permitting arbitrary symlink traversal inside the module.
		moduleDir := filepath.Clean(pkg.Module.Dir)
		moduleInfo, err := os.Stat(moduleDir)
		if err != nil {
			return nil, fmt.Errorf("capsule: stat main module directory: %w", err)
		}
		rootInfo, err := os.Stat(root)
		if err != nil {
			return nil, fmt.Errorf("capsule: stat workspace root: %w", err)
		}
		if !os.SameFile(moduleInfo, rootInfo) {
			return nil, fmt.Errorf("capsule: unexpected main module directory")
		}

		// Preserve the package path's original representation.
		// First try the canonical root; when Go reports an alias,
		// calculate the path relative to its own main-module root.
		dir := filepath.Clean(pkg.Dir)
		packageRel, err := filepath.Rel(root, dir)
		if err != nil || packageRel == ".." ||
			strings.HasPrefix(packageRel, ".."+string(filepath.Separator)) {
			packageRel, err = filepath.Rel(moduleDir, dir)
		}
		if err != nil {
			return nil, fmt.Errorf("capsule: resolve Go package directory: %w", err)
		}

		if packageRel == ".." ||
			strings.HasPrefix(packageRel, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("capsule: Go package outside workspace")
		}

		// Check every package-directory component without following
		// symbolic links. This is a metadata preflight, not an
		// atomic filesystem snapshot.
		current := root
		rootInfo, err = os.Lstat(current)
		if err != nil {
			return nil, fmt.Errorf("capsule: inspect workspace root: %w", err)
		}
		if !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("capsule: invalid workspace root")
		}

		if packageRel != "." {
			for _, component := range strings.Split(packageRel, string(filepath.Separator)) {
				current = filepath.Join(current, component)
				info, err := os.Lstat(current)
				if err != nil {
					return nil, fmt.Errorf("capsule: inspect Go package directory: %w", err)
				}
				if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
					return nil, fmt.Errorf(
						"capsule: invalid Go package directory: %q", current,
					)
				}
			}
		}

		for _, embedded := range pkg.EmbedFiles {
			// Validate the original metadata before filepath.Join
			// can normalize traversal, dot components or empty names.
			if err := ValidateWorkspacePath(embedded, 0600); err != nil {
				return nil, fmt.Errorf(
					"capsule: invalid Go embedded source path %q: %w",
					embedded, err,
				)
			}

			rel := filepath.ToSlash(
				filepath.Join(packageRel, filepath.FromSlash(embedded)),
			)

			// Validate the resulting workspace-relative path too.
			if err := ValidateWorkspacePath(rel, 0600); err != nil {
				return nil, fmt.Errorf(
					"capsule: invalid embedded workspace path %q: %w",
					rel, err,
				)
			}

			if _, exists := paths[rel]; !exists &&
				len(paths) >= MaxWorkspaceV3Files {
				return nil, fmt.Errorf("capsule: too many embedded files")
			}

			paths[rel] = struct{}{}
		}
	}

	if limited.N == 0 {
		return nil, fmt.Errorf("capsule: Go embed metadata exceeds output limit")
	}
	if !found {
		return nil, fmt.Errorf("capsule: no Go package metadata")
	}
	if !foundMainModule {
		return nil, fmt.Errorf("capsule: no main-module Go packages found")
	}

	result := make([]string, 0, len(paths))
	for name := range paths {
		result = append(result, name)
	}
	sort.Strings(result)

	return result, nil
}
