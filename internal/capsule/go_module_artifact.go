package capsule

import (
	"encoding/hex"
	"fmt"
	"strings"

	"golang.org/x/mod/module"
)

const (
	MaxGoModulesV3                  = 8
	MaxGoModuleArtifactsV3          = 3
	MaxGoModuleTotalSizeV3    int64 = 16 << 20
	MaxGoModuleUnpackedSizeV3 int64 = 16 << 20
)

type GoModuleArtifact struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type GoModuleV3 struct {
	Path      string             `json:"path"`
	Version   string             `json:"version"`
	Artifacts []GoModuleArtifact `json:"artifacts"`
}

func ValidateGoModulesV3(modules []GoModuleV3) error {
	if len(modules) > MaxGoModulesV3 {
		return fmt.Errorf("capsule: too many Go modules")
	}

	var totalSize int64
	var previousPath string

	for i, mod := range modules {
		if err := module.Check(mod.Path, mod.Version); err != nil {
			return fmt.Errorf("capsule: invalid Go module identity: %w", err)
		}

		if i > 0 && mod.Path <= previousPath {
			return fmt.Errorf(
				"capsule: Go modules must have unique, sorted paths",
			)
		}
		previousPath = mod.Path

		if len(mod.Artifacts) != MaxGoModuleArtifactsV3 {
			return fmt.Errorf(
				"capsule: Go module requires exactly three artifacts",
			)
		}

		prefix := fmt.Sprintf("module-%04d", i+1)

		expected := map[string]bool{
			prefix + ".zip":  false,
			prefix + ".mod":  false,
			prefix + ".info": false,
		}

		for _, artifact := range mod.Artifacts {
			seen, ok := expected[artifact.Path]
			if !ok {
				return fmt.Errorf(
					"capsule: unexpected Go artifact path %q",
					artifact.Path,
				)
			}

			if seen {
				return fmt.Errorf(
					"capsule: duplicate Go artifact %q",
					artifact.Path,
				)
			}
			expected[artifact.Path] = true

			if artifact.Size < 0 ||
				artifact.Size > MaxGoModuleTotalSizeV3-totalSize {
				return fmt.Errorf(
					"capsule: Go artifact size limit exceeded",
				)
			}

			if !strings.HasPrefix(artifact.SHA256, "sha256:") {
				return fmt.Errorf(
					"capsule: invalid Go artifact digest prefix",
				)
			}

			digest := strings.TrimPrefix(artifact.SHA256, "sha256:")
			if len(digest) != 64 {
				return fmt.Errorf(
					"capsule: invalid Go artifact SHA-256 length",
				)
			}

			if _, err := hex.DecodeString(digest); err != nil {
				return fmt.Errorf(
					"capsule: invalid Go artifact SHA-256: %w",
					err,
				)
			}

			totalSize += artifact.Size
		}
	}

	return nil
}
