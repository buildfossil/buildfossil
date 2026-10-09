package capsule

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func ReadWorkspaceFilesV2(root string, paths []string) ([]WorkspaceFile, error) {
	if len(paths) > MaxWorkspaceV2Files {
		return nil, fmt.Errorf("capsule: too many workspace files")
	}

	rootInfo, err := os.Lstat(root)
	if err != nil {
		return nil, fmt.Errorf("capsule: stat workspace root: %w", err)
	}
	if !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("capsule: invalid workspace root")
	}

	workspaceRoot, err := os.OpenRoot(root)
	if err != nil {
		return nil, fmt.Errorf("capsule: open workspace root: %w", err)
	}
	defer workspaceRoot.Close()

	files := make([]WorkspaceFile, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))
	var totalSize int64

	for _, name := range paths {
		if err := ValidateWorkspacePath(name, 0600); err != nil {
			return nil, fmt.Errorf("capsule: invalid workspace path: %w", err)
		}

		if _, exists := seen[name]; exists {
			return nil, fmt.Errorf("capsule: duplicate workspace path: %q", name)
		}
		seen[name] = struct{}{}

		for parent := path.Dir(name); parent != "."; parent = path.Dir(parent) {
			if _, exists := seen[parent]; exists {
				return nil, fmt.Errorf("capsule: file/directory path conflict: %q", parent)
			}
		}
		for existing := range seen {
			if existing != name && strings.HasPrefix(existing, name+"/") {
				return nil, fmt.Errorf("capsule: file/directory path conflict: %q", name)
			}
		}

		current := root
		parts := strings.Split(name, "/")

		for _, part := range parts[:len(parts)-1] {
			current = filepath.Join(current, part)

			info, err := os.Lstat(current)
			if err != nil {
				return nil, fmt.Errorf("capsule: inspect parent directory: %w", err)
			}
			if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				return nil, fmt.Errorf("capsule: invalid parent directory: %q", current)
			}
		}

		fullPath := filepath.Join(root, filepath.FromSlash(name))

		info, err := os.Lstat(fullPath)
		if err != nil {
			return nil, fmt.Errorf("capsule: stat workspace file: %w", err)
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("capsule: not a regular file: %q", name)
		}
		if info.Size() > MaxWorkspaceFileSize {
			return nil, fmt.Errorf("capsule: workspace file too large: %q", name)
		}
		if info.Size() > MaxWorkspaceV2TotalSize-totalSize {
			return nil, fmt.Errorf("capsule: workspace total size exceeded")
		}

		f, err := workspaceRoot.OpenFile(name, os.O_RDONLY, 0)
		if err != nil {
			return nil, fmt.Errorf("capsule: open workspace file: %w", err)
		}

		openedInfo, err := f.Stat()
		if err != nil {
			f.Close()
			return nil, fmt.Errorf("capsule: stat opened file: %w", err)
		}
		if !openedInfo.Mode().IsRegular() {
			f.Close()
			return nil, fmt.Errorf("capsule: opened file is not regular: %q", name)
		}

		data, readErr := io.ReadAll(io.LimitReader(f, MaxWorkspaceFileSize+1))
		closeErr := f.Close()

		if readErr != nil {
			return nil, fmt.Errorf("capsule: read workspace file: %w", readErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("capsule: close workspace file: %w", closeErr)
		}
		if int64(len(data)) > MaxWorkspaceFileSize {
			return nil, fmt.Errorf("capsule: workspace file too large: %q", name)
		}
		if int64(len(data)) > MaxWorkspaceV2TotalSize-totalSize {
			return nil, fmt.Errorf("capsule: workspace total size exceeded")
		}

		totalSize += int64(len(data))

		files = append(files, WorkspaceFile{
			Path: name,
			Data: data,
			Mode: 0600,
		})
	}

	return files, nil
}
