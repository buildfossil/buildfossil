package capsule

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/mod/module"
)

// PrepareGoModuleCacheV3 writes verified Go module artifacts into
// the standard Go module download-cache layout.
//
// cacheRoot must exist and must be empty.
// The returned directory is suitable for a read-only Docker bind mount.
func PrepareGoModuleCacheV3(
	cacheRoot string,
	verified VerifiedCapsuleV3,
) error {
	if verified.Manifest.SchemaVersion != SchemaVersionV3 {
		return fmt.Errorf("capsule: module cache requires schema v3")
	}

	if err := verified.Manifest.Validate(); err != nil {
		return fmt.Errorf("capsule: invalid manifest: %w", err)
	}

	if len(verified.Manifest.GoModules) != 1 {
		return fmt.Errorf("capsule: expected exactly one Go module")
	}

	mod := verified.Manifest.GoModules[0]

	var goSum []byte
	foundGoSum := false

	for i, file := range verified.Files {
		if i >= len(verified.Manifest.Workspace.Files) {
			return fmt.Errorf("capsule: unexpected workspace file")
		}

		metadata := verified.Manifest.Workspace.Files[i]

		if file.Path != metadata.Path {
			return fmt.Errorf("capsule: workspace path mismatch")
		}

		if err := VerifyWorkspaceFileV2(metadata, file.Data); err != nil {
			return err
		}

		if file.Path == "go.sum" {
			goSum = file.Data
			foundGoSum = true
		}
	}

	if len(verified.Files) != len(verified.Manifest.Workspace.Files) {
		return fmt.Errorf("capsule: workspace file count mismatch")
	}

	if !foundGoSum {
		return fmt.Errorf("capsule: missing go.sum")
	}

	if err := VerifyGoModuleArtifactsV3(
		mod,
		verified.Artifacts,
		goSum,
	); err != nil {
		return fmt.Errorf("capsule: invalid module artifacts: %w", err)
	}

	escapedPath, err := module.EscapePath(mod.Path)
	if err != nil {
		return fmt.Errorf("capsule: escape module path: %w", err)
	}

	escapedVersion, err := module.EscapeVersion(mod.Version)
	if err != nil {
		return fmt.Errorf("capsule: escape module version: %w", err)
	}

	info, err := os.Lstat(cacheRoot)
	if err != nil {
		return fmt.Errorf("capsule: inspect cache root: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("capsule: invalid cache root")
	}

	entries, err := os.ReadDir(cacheRoot)
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return fmt.Errorf("capsule: cache root must be empty")
	}

	relativeDir := filepath.Join(
		"cache",
		"download",
		filepath.FromSlash(escapedPath),
		"@v",
	)

	root, err := os.OpenRoot(cacheRoot)
	if err != nil {
		return fmt.Errorf("capsule: open cache root: %w", err)
	}
	defer root.Close()

	if err := root.MkdirAll(relativeDir, 0700); err != nil {
		return fmt.Errorf("capsule: create module cache directories: %w", err)
	}

	for _, ext := range []string{".zip", ".mod", ".info"} {
		source := "module-0001" + ext
		data := verified.Artifacts[source]

		destination := filepath.Join(relativeDir, escapedVersion+ext)

		f, err := root.OpenFile(
			destination,
			os.O_WRONLY|os.O_CREATE|os.O_EXCL,
			0600,
		)
		if err != nil {
			return fmt.Errorf("capsule: create cache artifact: %w", err)
		}

		_, writeErr := f.Write(data)
		closeErr := f.Close()

		if writeErr != nil {
			return fmt.Errorf("capsule: write cache artifact: %w", writeErr)
		}
		if closeErr != nil {
			return fmt.Errorf("capsule: close cache artifact: %w", closeErr)
		}
	}

	return nil
}
