package capsule

import (
	"encoding/hex"
	"fmt"
	"strings"

	"golang.org/x/mod/module"
)

const (
	MaxGoModulesV3                  = 1
	MaxGoModuleArtifactsV3          = 3
	MaxGoModuleTotalSizeV3    int64 = 1 << 20
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

	if len(modules) == 0 {
		return nil
	}

	var totalSize int64

	for _, mod := range modules {
		if err := module.Check(mod.Path, mod.Version); err != nil {
			return fmt.Errorf("capsule: invalid Go module identity: %w", err)
		}

		if len(mod.Artifacts) != MaxGoModuleArtifactsV3 {
			return fmt.Errorf("capsule: Go module requires exactly three artifacts")
		}

		expected := map[string]bool{
			"module-0001.zip":  false,
			"module-0001.mod":  false,
			"module-0001.info": false,
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
				return fmt.Errorf("capsule: invalid Go artifact digest prefix")
			}

			digest := strings.TrimPrefix(artifact.SHA256, "sha256:")
			if len(digest) != 64 {
				return fmt.Errorf("capsule: invalid Go artifact SHA-256 length")
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
