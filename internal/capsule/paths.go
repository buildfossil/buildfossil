package capsule

import (
	"errors"
	"io/fs"
	"path"
	"strings"
)

func ValidateWorkspacePath(name string, mode fs.FileMode) error {
	if name == "" || name == "." {
		return errors.New("capsule: empty workspace path")
	}

	if strings.ContainsRune(name, '\\') {
		return errors.New("capsule: backslash in workspace path")
	}

	if strings.ContainsRune(name, 0) {
		return errors.New("capsule: NUL in workspace path")
	}

	if strings.HasPrefix(name, "/") {
		return errors.New("capsule: absolute workspace path")
	}

	if path.Clean(name) != name {
		return errors.New("capsule: non-canonical workspace path")
	}

	for _, part := range strings.Split(name, "/") {
		if part == ".." || part == "." || part == "" {
			return errors.New("capsule: invalid workspace path component")
		}
	}

	if !mode.IsRegular() {
		return errors.New("capsule: only regular files are supported")
	}

	return nil
}
