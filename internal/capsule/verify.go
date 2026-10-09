package capsule

import (
	"errors"
	"fmt"
)

func VerifyWorkspaceFile(metadata FileMetadata, data []byte) error {
	if metadata.Path != "fixture.txt" {
		return errors.New("capsule: unsupported workspace file")
	}

	if metadata.Size < 0 || metadata.Size > MaxWorkspaceFileSize {
		return errors.New("capsule: invalid workspace file size")
	}

	if int64(len(data)) != metadata.Size {
		return fmt.Errorf(
			"capsule: file size mismatch: expected %d, got %d",
			metadata.Size,
			len(data),
		)
	}

	actual := SHA256(data)

	if actual != metadata.SHA256 {
		return fmt.Errorf("capsule: workspace SHA-256 mismatch")
	}

	return nil
}
