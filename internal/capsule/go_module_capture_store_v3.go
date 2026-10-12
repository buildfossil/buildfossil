package capsule

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/mod/module"
)

// ReadGoModuleArtifactSetToStoreV3 copies selected Go module
// artifacts from the local cache to an existing private store.
//
// No downloads are performed. ZIP contents are never accumulated
// in memory. The caller owns and closes the store.
//
// Use a fresh store; partial artifacts on error belong to the caller.
func ReadGoModuleArtifactSetToStoreV3(
	modules []GoModuleIdentityV3,
	goSum []byte,
	moduleCacheRoot string,
	store *ArtifactStoreV3,
) ([]GoModuleV3, error) {
	if len(modules) > MaxGoModulesV3 {
		return nil, fmt.Errorf("capsule: too many Go modules")
	}
	if moduleCacheRoot == "" {
		return nil, fmt.Errorf("capsule: missing module cache root")
	}
	if store == nil || store.closed {
		return nil, fmt.Errorf("capsule: unavailable artifact store")
	}
	if len(store.names) != 0 {
		return nil, fmt.Errorf("capsule: expected empty artifact store")
	}

	root, err := os.OpenRoot(moduleCacheRoot)
	if err != nil {
		return nil, fmt.Errorf("capsule: open Go module cache: %w", err)
	}
	defer root.Close()

	result := make([]GoModuleV3, 0, len(modules))
	var totalSize int64
	var previousPath string

	for index, identity := range modules {
		if err := module.Check(identity.Path, identity.Version); err != nil {
			return nil, fmt.Errorf("capsule: invalid selected module: %w", err)
		}
		if index > 0 && identity.Path <= previousPath {
			return nil, fmt.Errorf(
				"capsule: selected Go modules must be unique and sorted",
			)
		}
		previousPath = identity.Path

		escapedPath, err := module.EscapePath(identity.Path)
		if err != nil {
			return nil, err
		}
		escapedVersion, err := module.EscapeVersion(identity.Version)
		if err != nil {
			return nil, err
		}

		mod := GoModuleV3{
			Path:      identity.Path,
			Version:   identity.Version,
			Artifacts: make([]GoModuleArtifact, 0, MaxGoModuleArtifactsV3),
		}

		for _, ext := range []string{".zip", ".mod", ".info"} {
			source := filepath.Join(
				"cache", "download",
				filepath.FromSlash(escapedPath),
				"@v", escapedVersion+ext,
			)
			name := fmt.Sprintf("module-%04d%s", index+1, ext)

			if err := validateRegularCachePathV3(root, source); err != nil {
				return nil, err
			}

			file, err := root.Open(source)
			if err != nil {
				return nil, fmt.Errorf(
					"capsule: open cached artifact %s: %w", source, err,
				)
			}

			info, statErr := file.Stat()
			if statErr != nil {
				file.Close()
				return nil, statErr
			}

			remaining := MaxGoModuleTotalSizeV3 - totalSize
			if !info.Mode().IsRegular() ||
				info.Size() < 0 ||
				info.Size() > remaining {
				file.Close()
				return nil, fmt.Errorf(
					"capsule: invalid module artifact type or size: %s",
					source,
				)
			}

			hash := sha256.New()
			measured, hashErr := io.Copy(
				hash, io.LimitReader(file, info.Size()+1),
			)
			if hashErr != nil || measured != info.Size() {
				file.Close()
				return nil, fmt.Errorf(
					"capsule: module artifact changed during hashing: %s: %v",
					source, hashErr,
				)
			}

			if _, err := file.Seek(0, io.SeekStart); err != nil {
				file.Close()
				return nil, err
			}

			digest := hex.EncodeToString(hash.Sum(nil))
			putErr := store.Put(name, file, info.Size(), digest)
			closeErr := file.Close()

			if putErr != nil {
				return nil, fmt.Errorf(
					"capsule: store cached artifact %s: %w", source, putErr,
				)
			}
			if closeErr != nil {
				return nil, closeErr
			}

			mod.Artifacts = append(mod.Artifacts, GoModuleArtifact{
				Path:   name,
				Size:   info.Size(),
				SHA256: "sha256:" + digest,
			})
			totalSize += info.Size()
		}

		result = append(result, mod)
	}

	if err := verifyGoModuleSetFromStoreV3(
		result, goSum, store,
	); err != nil {
		return nil, fmt.Errorf(
			"capsule: verify cached Go module set: %w", err,
		)
	}

	return result, nil
}
