package capsule

import (
	"fmt"
	"io/fs"
	"os"
)

const MaxWorkspaceFileSize int64 = 10 << 20 // 10 MiB

type WorkspaceFile struct {
	Path string
	Data []byte
	Mode fs.FileMode
}

func ReadWorkspaceFile(root, name string) (WorkspaceFile, error) {
	if err := ValidateWorkspacePath(name, 0644); err != nil {
		return WorkspaceFile{}, err
	}

	// Reading arbitrary filesystem paths is not yet supported.
	// This function is restricted to a controlled test fixture.
	if name != "fixture.txt" {
		return WorkspaceFile{}, fmt.Errorf(
			"capsule: unsupported experimental file: %q", name,
		)
	}

	fullPath := root + string(os.PathSeparator) + name

	info, err := os.Lstat(fullPath)
	if err != nil {
		return WorkspaceFile{}, fmt.Errorf("capsule: stat file: %w", err)
	}

	if err := ValidateWorkspacePath(name, info.Mode()); err != nil {
		return WorkspaceFile{}, err
	}

	if info.Size() > MaxWorkspaceFileSize {
		return WorkspaceFile{}, fmt.Errorf("capsule: workspace file too large")
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return WorkspaceFile{}, fmt.Errorf("capsule: read file: %w", err)
	}

	if int64(len(data)) > MaxWorkspaceFileSize {
		return WorkspaceFile{}, fmt.Errorf("capsule: workspace file too large")
	}

	return WorkspaceFile{
		Path: name,
		Data: data,
		Mode: info.Mode().Perm(),
	}, nil
}
