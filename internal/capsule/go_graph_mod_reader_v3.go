package capsule

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/mod/module"
)

// ReadGoGraphModFilesForCaptureV3 reads additional go.mod files
// from the local Go module cache without network access.
func ReadGoGraphModFilesForCaptureV3(
	identities []GoModuleIdentityV3,
	goSum []byte,
	moduleCacheRoot string,
) ([]GoGraphModFileV3, map[string][]byte, error) {
	if len(identities) > MaxGoGraphModFilesV3 {
		return nil, nil, fmt.Errorf("capsule: too many graph mod files")
	}
	if moduleCacheRoot == "" {
		return nil, nil, fmt.Errorf("capsule: missing module cache root")
	}

	root, err := os.OpenRoot(moduleCacheRoot)
	if err != nil {
		return nil, nil, fmt.Errorf("capsule: open module cache: %w", err)
	}
	defer root.Close()

	files := make([]GoGraphModFileV3, 0, len(identities))
	artifacts := make(map[string][]byte, len(identities))

	var totalSize int64

	for i, identity := range identities {
		if err := module.Check(identity.Path, identity.Version); err != nil {
			return nil, nil, fmt.Errorf("capsule: invalid graph module: %w", err)
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
			"@v", escapedVersion+".mod",
		)

		if err := validateRegularCachePathV3(root, source); err != nil {
			return nil, nil, err
		}

		f, err := root.Open(source)
		if err != nil {
			return nil, nil, fmt.Errorf(
				"capsule: missing graph module metadata %s: %w",
				source, err,
			)
		}

		info, err := f.Stat()
		if err != nil {
			f.Close()
			return nil, nil, err
		}

		remaining := MaxGoGraphModTotalSizeV3 - totalSize

		if !info.Mode().IsRegular() ||
			info.Size() > remaining {
			f.Close()
			return nil, nil, fmt.Errorf(
				"capsule: graph metadata size limit exceeded",
			)
		}

		data, readErr := io.ReadAll(io.LimitReader(f, remaining+1))
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
				"capsule: graph metadata changed or exceeded limit",
			)
		}

		name := fmt.Sprintf("graph-mod-%04d.mod", i+1)

		file := GoGraphModFileV3{
			Path:    identity.Path,
			Version: identity.Version,
			Artifact: GoModuleArtifact{
				Path:   name,
				Size:   int64(len(data)),
				SHA256: SHA256(data),
			},
		}

		if err := VerifyGoGraphModFileV3(file, data, goSum); err != nil {
			return nil, nil, fmt.Errorf(
				"capsule: invalid graph metadata %s@%s: %w",
				identity.Path, identity.Version, err,
			)
		}

		totalSize += int64(len(data))
		files = append(files, file)
		artifacts[name] = data
	}

	if err := ValidateGoGraphModFilesV3(files); err != nil {
		return nil, nil, err
	}

	return files, artifacts, nil
}
