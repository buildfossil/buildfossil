package capsule

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

// VerifyStagedWorkspaceV3 checks the restored workspace immediately
// before Docker receives it as a bind mount.
func VerifyStagedWorkspaceV3(
	root string,
	manifest Manifest,
) error {
	if manifest.SchemaVersion != SchemaVersionV3 {
		return fmt.Errorf("capsule: staging verification requires v3")
	}

	if err := manifest.Validate(); err != nil {
		return fmt.Errorf("capsule: invalid staged manifest: %w", err)
	}

	expected := make(map[string]FileMetadata, len(manifest.Workspace.Files))
	paths := make([]string, 0, len(manifest.Workspace.Files))

	for _, metadata := range manifest.Workspace.Files {
		expected[metadata.Path] = metadata
		paths = append(paths, metadata.Path)
	}

	// Reject unexpected files, directories that aren't required by
	// manifest paths, and symbolic links anywhere in the workspace.
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if path == root {
			return nil
		}

		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		relative = filepath.ToSlash(relative)

		if entry.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("capsule: symlink in staged workspace: %s", relative)
		}

		if entry.IsDir() {
			for expectedPath := range expected {
				if strings.HasPrefix(expectedPath, relative+"/") {
					return nil
				}
			}

			return fmt.Errorf(
				"capsule: unexpected staged directory: %s",
				relative,
			)
		}

		if !entry.Type().IsRegular() {
			return fmt.Errorf(
				"capsule: unsupported staged file type: %s",
				relative,
			)
		}

		if _, ok := expected[relative]; !ok {
			return fmt.Errorf(
				"capsule: unexpected staged file: %s",
				relative,
			)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("capsule: walk staged workspace: %w", err)
	}

	files, err := ReadWorkspaceFilesV2(root, paths)
	if err != nil {
		return fmt.Errorf("capsule: reread staged workspace: %w", err)
	}

	for _, file := range files {
		metadata := expected[file.Path]

		if err := VerifyWorkspaceFileV2(metadata, file.Data); err != nil {
			return fmt.Errorf(
				"capsule: staged file %s failed verification: %w",
				file.Path,
				err,
			)
		}
	}

	return nil
}
