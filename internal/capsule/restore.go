package capsule

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func RestoreWorkspace(root string, verified VerifiedCapsule) error {
	if verified.File.Path != "fixture.txt" {
		return errors.New("capsule: unsupported restore path")
	}

	if len(verified.Manifest.Workspace.Files) != 1 {
		return errors.New("capsule: invalid workspace metadata")
	}

	metadata := verified.Manifest.Workspace.Files[0]

	if err := VerifyWorkspaceFile(metadata, verified.File.Data); err != nil {
		return fmt.Errorf("capsule: restore verification: %w", err)
	}

	info, err := os.Lstat(root)
	if err != nil {
		return fmt.Errorf("capsule: stat restore directory: %w", err)
	}

	if !info.IsDir() {
		return errors.New("capsule: restore destination is not a directory")
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return fmt.Errorf("capsule: inspect restore directory: %w", err)
	}

	if len(entries) != 0 {
		return errors.New("capsule: restore directory must be empty")
	}

	destination := filepath.Join(root, "fixture.txt")

	f, err := os.OpenFile(
		destination,
		os.O_WRONLY|os.O_CREATE|os.O_EXCL,
		0600,
	)
	if err != nil {
		return fmt.Errorf("capsule: create restored file: %w", err)
	}

	if _, err := f.Write(verified.File.Data); err != nil {
		f.Close()
		os.Remove(destination)
		return fmt.Errorf("capsule: write restored file: %w", err)
	}

	if err := f.Close(); err != nil {
		os.Remove(destination)
		return fmt.Errorf("capsule: close restored file: %w", err)
	}

	return nil
}
