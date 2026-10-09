package capsule

import (
	"fmt"
	"os"
	"path/filepath"
)

func WriteFile(output string, manifest Manifest, file WorkspaceFile) (err error) {
	if output == "" {
		return fmt.Errorf("capsule: empty output path")
	}

	if _, statErr := os.Lstat(output); statErr == nil {
		return fmt.Errorf("capsule: output already exists: %s", output)
	} else if !os.IsNotExist(statErr) {
		return fmt.Errorf("capsule: check output: %w", statErr)
	}

	dir := filepath.Dir(output)

	temp, err := os.CreateTemp(dir, ".buildfossil-*.tmp")
	if err != nil {
		return fmt.Errorf("capsule: create temporary file: %w", err)
	}

	tempPath := temp.Name()
	defer os.Remove(tempPath)

	if err := temp.Chmod(0600); err != nil {
		temp.Close()
		return fmt.Errorf("capsule: set permissions: %w", err)
	}

	if err := WriteWithWorkspace(temp, manifest, file); err != nil {
		temp.Close()
		return fmt.Errorf("capsule: write archive: %w", err)
	}

	if err := temp.Sync(); err != nil {
		temp.Close()
		return fmt.Errorf("capsule: sync archive: %w", err)
	}

	if err := temp.Close(); err != nil {
		return fmt.Errorf("capsule: close archive: %w", err)
	}

	if err := os.Link(tempPath, output); err != nil {
		return fmt.Errorf("capsule: publish archive: %w", err)
	}

	return nil
}
