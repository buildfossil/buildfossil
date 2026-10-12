package capsule

import (
	"encoding/hex"
	"fmt"
	"strings"

	"golang.org/x/mod/module"
)

const (
	MaxGoGraphModFilesV3           = 128
	MaxGoGraphModTotalSizeV3 int64 = 1 << 20
)

type GoGraphModFileV3 struct {
	Path     string           `json:"path"`
	Version  string           `json:"version"`
	Artifact GoModuleArtifact `json:"artifact"`
}

// ValidateGoGraphModFilesV3 validates metadata for additional go.mod
// files required to reconstruct a Go module graph offline.
//
// These files are separate from selected modules and must contain
// only module metadata, never ZIP archives or executable content.
func ValidateGoGraphModFilesV3(files []GoGraphModFileV3) error {
	if len(files) > MaxGoGraphModFilesV3 {
		return fmt.Errorf("capsule: too many Go graph mod files")
	}

	var totalSize int64
	var previousPath string
	var previousVersion string

	for i, file := range files {
		if err := module.Check(file.Path, file.Version); err != nil {
			return fmt.Errorf(
				"capsule: invalid Go graph module identity: %w", err,
			)
		}

		if i > 0 {
			if file.Path < previousPath ||
				(file.Path == previousPath && file.Version <= previousVersion) {
				return fmt.Errorf(
					"capsule: Go graph mod files must be uniquely sorted",
				)
			}
		}

		previousPath = file.Path
		previousVersion = file.Version

		expectedName := fmt.Sprintf("graph-mod-%04d.mod", i+1)
		if file.Artifact.Path != expectedName {
			return fmt.Errorf(
				"capsule: invalid Go graph artifact path %q",
				file.Artifact.Path,
			)
		}

		if file.Artifact.Size < 0 ||
			file.Artifact.Size > MaxGoGraphModTotalSizeV3-totalSize {
			return fmt.Errorf(
				"capsule: Go graph metadata size limit exceeded",
			)
		}

		if !strings.HasPrefix(file.Artifact.SHA256, "sha256:") {
			return fmt.Errorf(
				"capsule: invalid Go graph metadata digest prefix",
			)
		}

		digest := strings.TrimPrefix(file.Artifact.SHA256, "sha256:")
		if len(digest) != 64 {
			return fmt.Errorf(
				"capsule: invalid Go graph metadata SHA-256 length",
			)
		}

		if _, err := hex.DecodeString(digest); err != nil {
			return fmt.Errorf(
				"capsule: invalid Go graph metadata SHA-256: %w", err,
			)
		}

		totalSize += file.Artifact.Size
	}

	return nil
}
