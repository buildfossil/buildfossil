package capsule

import (
	"fmt"
	"os"
	"path"
	"strings"
)

func RestoreWorkspaceV2(root string, verified VerifiedCapsuleV2) error {
	if verified.Manifest.SchemaVersion != SchemaVersionV2 {
		return fmt.Errorf("capsule: restore requires schema version 2")
	}

	return restoreWorkspaceFiles(
		root,
		verified.Manifest,
		verified.Files,
	)
}

func restoreWorkspaceFiles(
	root string,
	manifest Manifest,
	files []WorkspaceFile,
) error {
	if manifest.SchemaVersion != SchemaVersionV2 &&
		manifest.SchemaVersion != SchemaVersionV3 {
		return fmt.Errorf("capsule: unsupported workspace restore schema")
	}

	if err := manifest.Validate(); err != nil {
		return fmt.Errorf("capsule: invalid manifest: %w", err)
	}

	metadata := manifest.Workspace.Files

	if len(files) != len(metadata) {
		return fmt.Errorf("capsule: workspace file count mismatch")
	}

	// Verify all files before writing anything to disk.
	for i, file := range files {
		if file.Path != metadata[i].Path {
			return fmt.Errorf("capsule: workspace path mismatch")
		}

		if err := ValidateWorkspacePath(file.Path, file.Mode); err != nil {
			return fmt.Errorf("capsule: invalid file: %w", err)
		}

		if err := VerifyWorkspaceFileV2(metadata[i], file.Data); err != nil {
			return fmt.Errorf("capsule: verify file: %w", err)
		}
	}

	// Reject file/directory path conflicts.
	paths := make(map[string]struct{}, len(files))
	for _, file := range files {
		paths[file.Path] = struct{}{}
	}

	for _, file := range files {
		parts := strings.Split(file.Path, "/")
		for i := 1; i < len(parts); i++ {
			parent := strings.Join(parts[:i], "/")
			if _, exists := paths[parent]; exists {
				return fmt.Errorf(
					"capsule: file/directory path conflict: %q",
					parent,
				)
			}
		}
	}

	info, err := os.Lstat(root)
	if err != nil {
		return fmt.Errorf("capsule: stat restore directory: %w", err)
	}

	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("capsule: invalid restore directory")
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return fmt.Errorf("capsule: inspect restore directory: %w", err)
	}

	if len(entries) != 0 {
		return fmt.Errorf("capsule: restore directory must be empty")
	}

	workspaceRoot, err := os.OpenRoot(root)
	if err != nil {
		return fmt.Errorf("capsule: open restore root: %w", err)
	}
	defer workspaceRoot.Close()

	for _, file := range files {
		parent := path.Dir(file.Path)

		if parent != "." {
			parts := strings.Split(parent, "/")
			current := ""

			for _, part := range parts {
				if current == "" {
					current = part
				} else {
					current += "/" + part
				}

				err := workspaceRoot.Mkdir(current, 0700)
				if err != nil && !os.IsExist(err) {
					return fmt.Errorf(
						"capsule: create restore directory %q: %w",
						current,
						err,
					)
				}

				info, err := workspaceRoot.Lstat(current)
				if err != nil {
					return fmt.Errorf(
						"capsule: inspect restore directory: %w",
						err,
					)
				}

				if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
					return fmt.Errorf(
						"capsule: invalid restore directory: %q",
						current,
					)
				}
			}
		}

		f, err := workspaceRoot.OpenFile(
			file.Path,
			os.O_WRONLY|os.O_CREATE|os.O_EXCL,
			0600,
		)
		if err != nil {
			return fmt.Errorf(
				"capsule: create restored file %q: %w",
				file.Path,
				err,
			)
		}

		if _, err := f.Write(file.Data); err != nil {
			f.Close()
			return fmt.Errorf(
				"capsule: write restored file %q: %w",
				file.Path,
				err,
			)
		}

		if err := f.Close(); err != nil {
			return fmt.Errorf(
				"capsule: close restored file %q: %w",
				file.Path,
				err,
			)
		}
	}

	return nil
}
