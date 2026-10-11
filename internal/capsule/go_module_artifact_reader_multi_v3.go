package capsule

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/mod/module"
)

// ReadGoModuleArtifactSetForCaptureV3 reads selected Go module
// artifacts from the local module download cache.
//
// It never downloads modules or accesses the network.
// Modules must have unique paths in ascending order.
func ReadGoModuleArtifactSetForCaptureV3(
	modules []GoModuleIdentityV3,
	goSum []byte,
	moduleCacheRoot string,
) ([]GoModuleV3, map[string][]byte, error) {
	if len(modules) > MaxGoModulesV3 {
		return nil, nil, fmt.Errorf("capsule: too many Go modules")
	}

	if moduleCacheRoot == "" {
		return nil, nil, fmt.Errorf("capsule: missing module cache root")
	}

	root, err := os.OpenRoot(moduleCacheRoot)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"capsule: open Go module cache: %w", err,
		)
	}
	defer root.Close()

	result := make([]GoModuleV3, 0, len(modules))
	artifacts := make(map[string][]byte, len(modules)*3)

	var totalSize int64
	var previousPath string

	for i, identity := range modules {
		if err := module.Check(identity.Path, identity.Version); err != nil {
			return nil, nil, fmt.Errorf(
				"capsule: invalid selected module: %w", err,
			)
		}

		if i > 0 && identity.Path <= previousPath {
			return nil, nil, fmt.Errorf(
				"capsule: selected Go modules must be unique and sorted",
			)
		}
		previousPath = identity.Path

		escapedPath, err := module.EscapePath(identity.Path)
		if err != nil {
			return nil, nil, err
		}

		escapedVersion, err := module.EscapeVersion(identity.Version)
		if err != nil {
			return nil, nil, err
		}

		prefix := fmt.Sprintf("module-%04d", i+1)

		mod := GoModuleV3{
			Path:      identity.Path,
			Version:   identity.Version,
			Artifacts: make([]GoModuleArtifact, 0, 3),
		}

		for _, ext := range []string{".zip", ".mod", ".info"} {
			source := filepath.Join(
				"cache",
				"download",
				filepath.FromSlash(escapedPath),
				"@v",
				escapedVersion+ext,
			)

			name := prefix + ext

			if err := validateRegularCachePathV3(root, source); err != nil {
				return nil, nil, err
			}

			f, err := root.Open(source)
			if err != nil {
				return nil, nil, fmt.Errorf(
					"capsule: missing cached module artifact %s: %w",
					source, err,
				)
			}

			info, err := f.Stat()
			if err != nil {
				f.Close()
				return nil, nil, err
			}

			remaining := MaxGoModuleTotalSizeV3 - totalSize

			if !info.Mode().IsRegular() ||
				info.Size() > remaining {
				f.Close()
				return nil, nil, fmt.Errorf(
					"capsule: invalid module artifact type or size: %s",
					source,
				)
			}

			data, readErr := io.ReadAll(
				io.LimitReader(f, remaining+1),
			)
			closeErr := f.Close()

			if readErr != nil {
				return nil, nil, readErr
			}
			if closeErr != nil {
				return nil, nil, closeErr
			}

			if int64(len(data)) != info.Size() ||
				int64(len(data)) > remaining {
				return nil, nil, fmt.Errorf(
					"capsule: module artifact changed or exceeded limit: %s",
					source,
				)
			}

			totalSize += int64(len(data))

			artifacts[name] = data
			mod.Artifacts = append(
				mod.Artifacts,
				GoModuleArtifact{
					Path:   name,
					Size:   int64(len(data)),
					SHA256: SHA256(data),
				},
			)
		}

		result = append(result, mod)
	}

	if err := VerifyGoModuleArtifactSetV3(
		result,
		artifacts,
		goSum,
	); err != nil {
		return nil, nil, fmt.Errorf(
			"capsule: verify selected Go module artifacts: %w",
			err,
		)
	}

	return result, artifacts, nil
}
