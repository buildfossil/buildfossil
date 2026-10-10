package capsule

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
)

// ReadGoModuleArtifactsForCaptureV3 reads one direct Go dependency
// from the local Go module download cache.
//
// It does not download modules or access the network.
func ReadGoModuleArtifactsForCaptureV3(
	goMod []byte,
	goSum []byte,
	moduleCacheRoot string,
) (GoModuleV3, map[string][]byte, error) {
	var empty GoModuleV3

	parsed, err := modfile.Parse("go.mod", goMod, nil)
	if err != nil {
		return empty, nil, fmt.Errorf(
			"capsule: parse go.mod: %w", err,
		)
	}

	if len(parsed.Require) != 1 {
		return empty, nil, fmt.Errorf(
			"capsule: v3 capture requires exactly one Go dependency, found %d",
			len(parsed.Require),
		)
	}

	required := parsed.Require[0]

	if required.Indirect {
		return empty, nil, fmt.Errorf(
			"capsule: v3 capture requires a direct Go dependency",
		)
	}

	if len(parsed.Replace) != 0 || len(parsed.Exclude) != 0 {
		return empty, nil, fmt.Errorf(
			"capsule: replace/exclude directives are unsupported in v3",
		)
	}

	escapedPath, err := module.EscapePath(required.Mod.Path)
	if err != nil {
		return empty, nil, err
	}

	escapedVersion, err := module.EscapeVersion(required.Mod.Version)
	if err != nil {
		return empty, nil, err
	}

	artifacts := make(map[string][]byte, 3)
	metadata := make([]GoModuleArtifact, 0, 3)

	root, err := os.OpenRoot(moduleCacheRoot)
	if err != nil {
		return empty, nil, fmt.Errorf(
			"capsule: open module cache: %w", err,
		)
	}
	defer root.Close()

	var totalSize int64

	for _, ext := range []string{".zip", ".mod", ".info"} {
		name := "module-0001" + ext
		source := filepath.Join(
			"cache",
			"download",
			filepath.FromSlash(escapedPath),
			"@v",
			escapedVersion+ext,
		)

		f, err := root.Open(source)
		if err != nil {
			return empty, nil, fmt.Errorf(
				"capsule: open cached artifact %s: %w",
				source, err,
			)
		}

		info, err := f.Stat()
		if err != nil {
			f.Close()
			return empty, nil, err
		}

		remaining := MaxGoModuleTotalSizeV3 - totalSize

		if !info.Mode().IsRegular() ||
			info.Size() > remaining {
			f.Close()
			return empty, nil, fmt.Errorf(
				"capsule: invalid artifact type or size: %s",
				source,
			)
		}

		data, readErr := io.ReadAll(io.LimitReader(
			f,
			remaining+1,
		))
		closeErr := f.Close()

		if readErr != nil {
			return empty, nil, readErr
		}
		if closeErr != nil {
			return empty, nil, closeErr
		}

		if int64(len(data)) > remaining ||
			int64(len(data)) != info.Size() {
			return empty, nil, fmt.Errorf(
				"capsule: artifact size changed or exceeded limit",
			)
		}

		totalSize += int64(len(data))

		artifacts[name] = data

		metadata = append(metadata, GoModuleArtifact{
			Path:   name,
			Size:   int64(len(data)),
			SHA256: SHA256(data),
		})
	}

	mod := GoModuleV3{
		Path:      required.Mod.Path,
		Version:   required.Mod.Version,
		Artifacts: metadata,
	}

	if err := VerifyGoModuleArtifactsV3(
		mod,
		artifacts,
		goSum,
	); err != nil {
		return empty, nil, fmt.Errorf(
			"capsule: verify cached dependency: %w",
			err,
		)
	}

	return mod, artifacts, nil
}
