package capsule

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// validateRegularCachePathV3 rejects symlinks in every path
// component and requires the final component to be a regular file.
//
// The caller must still use root.Open to keep filesystem access
// confined to the module cache root.
func validateRegularCachePathV3(root *os.Root, name string) error {
	clean := filepath.Clean(name)

	if filepath.IsAbs(clean) ||
		clean == "." ||
		clean == ".." ||
		strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("capsule: invalid module cache path")
	}

	parts := strings.Split(clean, string(filepath.Separator))
	current := ""

	for i, part := range parts {
		if part == "" || part == "." || part == ".." {
			return fmt.Errorf("capsule: invalid module cache path component")
		}

		current = filepath.Join(current, part)

		info, err := root.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf(
					"capsule: missing cached module artifact %s: %w",
					name,
					err,
				)
			}

			return fmt.Errorf(
				"capsule: inspect cached artifact path %s: %w",
				current,
				err,
			)
		}

		if info.Mode()&fs.ModeSymlink != 0 {
			return fmt.Errorf("capsule: symlink in module cache path: %s", current)
		}

		if i == len(parts)-1 {
			if !info.Mode().IsRegular() {
				return fmt.Errorf("capsule: module artifact is not a regular file: %s", current)
			}
		} else if !info.IsDir() {
			return fmt.Errorf("capsule: module cache path component is not a directory: %s", current)
		}
	}

	return nil
}
