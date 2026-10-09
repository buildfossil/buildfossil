package capsule

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func RestoreWorkspaceV2(root string, verified VerifiedCapsuleV2) error {
	if verified.Manifest.SchemaVersion != SchemaVersionV2 {
		return fmt.Errorf("capsule: restore requires schema version 2")
	}

	if err := verified.Manifest.Validate(); err != nil {
		return fmt.Errorf("capsule: invalid manifest: %w", err)
	}

	metadata := verified.Manifest.Workspace.Files

	if len(verified.Files) != len(metadata) {
		return fmt.Errorf("capsule: workspace file count mismatch")
	}

	// Verify all files before writing anything to disk.
	for i, file := range verified.Files {
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
	paths := make(map[string]struct{}, len(verified.Files))
	for _, file := range verified.Files {
		paths[file.Path] = struct{}{}
	}

	for _, file := range verified.Files {
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

	for _, file := range verified.Files {
		destination := filepath.Join(root, filepath.FromSlash(file.Path))
		parent := filepath.Dir(destination)

		if err := os.MkdirAll(parent, 0700); err != nil {
			return fmt.Errorf("capsule: create directory: %w", err)
		}

		f, err := os.OpenFile(
			destination,
			os.O_WRONLY|os.O_CREATE|os.O_EXCL,
			0600,
		)
		if err != nil {
			return fmt.Errorf("capsule: create restored file: %w", err)
		}

		if _, err := f.Write(file.Data); err != nil {
			f.Close()
			return fmt.Errorf("capsule: write restored file: %w", err)
		}

		if err := f.Close(); err != nil {
			return fmt.Errorf("capsule: close restored file: %w", err)
		}
	}

	return nil
}
