package capsule

import (
	"encoding/hex"
	"fmt"
	"io/fs"
	"path"
	"strings"
)

const (
	MaxWorkspaceV2Files     = 64
	MaxWorkspaceV2TotalSize = 14 << 20
)
const MaxWorkspaceV2PathBytes = 90

func validateWorkspaceV2(workspace Workspace) error {
	return validateWorkspaceWithFileLimit(workspace, MaxWorkspaceV2Files, false)
}

func validateWorkspaceV3(workspace Workspace) error {
	return validateWorkspaceWithFileLimit(workspace, MaxWorkspaceV3Files, true)
}

func validateWorkspaceWithFileLimit(
	workspace Workspace,
	maxFiles int,
	allowUSTAR bool,
) error {
	if len(workspace.Files) > maxFiles {
		return fmt.Errorf("capsule: too many workspace files")
	}

	seen := make(map[string]struct{}, len(workspace.Files))
	parentDirs := make(map[string]struct{})
	var totalSize int64

	for _, file := range workspace.Files {
		if err := ValidateWorkspacePath(
			file.Path,
			fs.FileMode(0600),
		); err != nil {
			return err
		}

		if allowUSTAR {
			if err := validateWorkspaceUSTARPathV3(file.Path); err != nil {
				return err
			}
		} else if len("workspace/"+file.Path) > 100 {
			return fmt.Errorf(
				"capsule: workspace path exceeds canonical TAR limit: %q",
				file.Path,
			)
		}

		if _, exists := seen[file.Path]; exists {
			return fmt.Errorf(
				"capsule: duplicate workspace path: %q",
				file.Path,
			)
		}
		if _, exists := parentDirs[file.Path]; exists {
			return fmt.Errorf(
				"capsule: file/directory path conflict: %q",
				file.Path,
			)
		}

		for parent := path.Dir(file.Path); parent != "."; parent = path.Dir(parent) {
			if _, exists := seen[parent]; exists {
				return fmt.Errorf(
					"capsule: file/directory path conflict: %q",
					parent,
				)
			}
			parentDirs[parent] = struct{}{}
		}

		seen[file.Path] = struct{}{}

		if file.Size < 0 || file.Size > MaxWorkspaceFileSize {
			return fmt.Errorf(
				"capsule: invalid workspace file size: %q",
				file.Path,
			)
		}

		if file.Size > MaxWorkspaceV2TotalSize-totalSize {
			return fmt.Errorf("capsule: workspace total size exceeded")
		}
		totalSize += file.Size

		if !strings.HasPrefix(file.SHA256, "sha256:") {
			return fmt.Errorf("capsule: unsupported digest format")
		}

		digest := strings.TrimPrefix(file.SHA256, "sha256:")
		if len(digest) != 64 {
			return fmt.Errorf("capsule: invalid SHA-256 length")
		}

		if _, err := hex.DecodeString(digest); err != nil {
			return fmt.Errorf("capsule: invalid SHA-256: %w", err)
		}
	}

	return nil
}
