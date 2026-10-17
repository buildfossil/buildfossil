package capsule

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

// ResolveGoExecutable resolves the Go executable from PATH once.
// It does not establish filesystem immutability or verify the Go version.
func ResolveGoExecutable() (string, error) {
	path, err := exec.LookPath("go")
	if err != nil {
		return "", fmt.Errorf("capsule: locate Go executable: %w", err)
	}

	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("capsule: resolve Go executable path: %w", err)
	}

	return absolute, nil
}
