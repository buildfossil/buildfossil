package capsule

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/mod/module"
)

// ReadGoGraphInfoFilesForCaptureV3 reads metadata-only .info
// files from the local cache without downloading dependencies.
func ReadGoGraphInfoFilesForCaptureV3(
	selected []GoModuleIdentityV3,
	full []GoModuleV3,
	moduleCacheRoot string,
) ([]GoGraphInfoFileV3, map[string][]byte, error) {
	if moduleCacheRoot == "" {
		return nil, nil, fmt.Errorf("capsule: missing module cache root")
	}

	if len(selected) > MaxGoSelectedModulesV3 {
		return nil, nil, fmt.Errorf("capsule: too many selected modules")
	}

	root, err := os.OpenRoot(moduleCacheRoot)
	if err != nil {
		return nil, nil, fmt.Errorf("capsule: open module cache: %w", err)
	}
	defer root.Close()

	fullVersions := make(map[string]string, len(full))
	for _, mod := range full {
		if err := module.Check(mod.Path, mod.Version); err != nil {
			return nil, nil, err
		}
		fullVersions[mod.Path] = mod.Version
	}

	files := make([]GoGraphInfoFileV3, 0)
	artifacts := make(map[string][]byte)
	var total int64
	var previousPath string

	for _, identity := range selected {
		if err := module.Check(identity.Path, identity.Version); err != nil {
			return nil, nil, fmt.Errorf("capsule: invalid selected identity: %w", err)
		}

		if previousPath != "" && identity.Path <= previousPath {
			return nil, nil, fmt.Errorf("capsule: selected modules not sorted")
		}
		previousPath = identity.Path

		if version, found := fullVersions[identity.Path]; found {
			if version != identity.Version {
				return nil, nil, fmt.Errorf("capsule: full module version mismatch")
			}
			continue
		}

		escapedPath, err := module.EscapePath(identity.Path)
		if err != nil {
			return nil, nil, err
		}

		escapedVersion, err := module.EscapeVersion(identity.Version)
		if err != nil {
			return nil, nil, err
		}

		source := filepath.Join(
			"cache", "download",
			filepath.FromSlash(escapedPath),
			"@v", escapedVersion+".info",
		)

		if err := validateRegularCachePathV3(root, source); err != nil {
			return nil, nil, err
		}

		f, err := root.Open(source)
		if err != nil {
			return nil, nil, fmt.Errorf(
				"capsule: missing selected module info %s@%s: %w",
				identity.Path, identity.Version, err,
			)
		}

		stat, err := f.Stat()
		if err != nil {
			f.Close()
			return nil, nil, err
		}

		remaining := MaxGoGraphInfoTotalSizeV3 - total
		if !stat.Mode().IsRegular() ||
			stat.Size() > remaining {
			f.Close()
			return nil, nil, fmt.Errorf("capsule: graph info size limit")
		}

		data, readErr := io.ReadAll(io.LimitReader(f, remaining+1))
		closeErr := f.Close()

		if readErr != nil {
			return nil, nil, readErr
		}
		if closeErr != nil {
			return nil, nil, closeErr
		}
		if int64(len(data)) != stat.Size() ||
			int64(len(data)) > remaining {
			return nil, nil, fmt.Errorf("capsule: graph info changed during read")
		}

		index := len(files) + 1
		name := fmt.Sprintf("graph-info-%04d.info", index)

		file := GoGraphInfoFileV3{
			Path:    identity.Path,
			Version: identity.Version,
			Artifact: GoModuleArtifact{
				Path:   name,
				Size:   int64(len(data)),
				SHA256: SHA256(data),
			},
		}

		if err := VerifyGoGraphInfoFileV3(file, data); err != nil {
			return nil, nil, err
		}

		files = append(files, file)
		artifacts[name] = data
		total += int64(len(data))
	}

	if err := ValidateGoGraphInfoFilesV3(files, selected, full); err != nil {
		return nil, nil, err
	}

	if err := ValidateGoSelectedMetadataCompletenessV3(
		selected, full, files,
	); err != nil {
		return nil, nil, err
	}

	return files, artifacts, nil
}
