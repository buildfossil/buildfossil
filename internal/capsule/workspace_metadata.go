package capsule

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

func (w Workspace) Validate() error {
	if len(w.Files) > 1 {
		return errors.New("capsule: too many workspace files")
	}

	for _, file := range w.Files {
		if file.Path != "fixture.txt" {
			return fmt.Errorf("capsule: unsupported workspace path: %q", file.Path)
		}

		if err := ValidateWorkspacePath(file.Path, 0600); err != nil {
			return err
		}

		if file.Size < 0 || file.Size > MaxWorkspaceFileSize {
			return fmt.Errorf("capsule: invalid workspace file size")
		}

		if !strings.HasPrefix(file.SHA256, "sha256:") {
			return errors.New("capsule: unsupported digest format")
		}

		digest := strings.TrimPrefix(file.SHA256, "sha256:")

		if len(digest) != 64 {
			return errors.New("capsule: invalid SHA-256 length")
		}

		if _, err := hex.DecodeString(digest); err != nil {
			return fmt.Errorf("capsule: invalid SHA-256: %w", err)
		}
	}

	return nil
}
