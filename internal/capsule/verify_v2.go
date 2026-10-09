package capsule

import (
	"fmt"
	"io/fs"
)

func VerifyWorkspaceFileV2(metadata FileMetadata, data []byte) error {
	if err := ValidateWorkspacePath(metadata.Path, fs.FileMode(0600)); err != nil {
		return fmt.Errorf("capsule: invalid workspace path: %w", err)
	}

	if metadata.Size < 0 || metadata.Size > MaxWorkspaceFileSize {
		return fmt.Errorf("capsule: invalid workspace file size")
	}

	if int64(len(data)) != metadata.Size {
		return fmt.Errorf(
			"capsule: file size mismatch: expected %d, got %d",
			metadata.Size,
			len(data),
		)
	}

	if SHA256(data) != metadata.SHA256 {
		return fmt.Errorf("capsule: workspace SHA-256 mismatch")
	}

	return nil
}
