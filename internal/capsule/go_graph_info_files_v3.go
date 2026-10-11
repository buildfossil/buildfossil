package capsule

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"golang.org/x/mod/module"
)

const (
	MaxGoGraphInfoFilesV3           = MaxGoSelectedModulesV3
	MaxGoGraphInfoTotalSizeV3 int64 = 1 << 20
)

type GoGraphInfoFileV3 struct {
	Path     string           `json:"path"`
	Version  string           `json:"version"`
	Artifact GoModuleArtifact `json:"artifact"`
}

// ValidateGoGraphInfoFilesV3 validates metadata-only .info
// declarations against the complete selected module graph.
//
// Full ZIP modules already contain their own .info artifacts
// and must never be duplicated here.
func ValidateGoGraphInfoFilesV3(
	files []GoGraphInfoFileV3,
	selected []GoModuleIdentityV3,
	full []GoModuleV3,
) error {
	if len(files) > MaxGoGraphInfoFilesV3 {
		return fmt.Errorf("capsule: too many graph info files")
	}

	selectedVersions := make(map[string]string, len(selected))
	for _, mod := range selected {
		selectedVersions[mod.Path] = mod.Version
	}

	fullVersions := make(map[string]string, len(full))
	for _, mod := range full {
		fullVersions[mod.Path] = mod.Version
	}

	var total int64
	var previousPath string

	for i, file := range files {
		if err := module.Check(file.Path, file.Version); err != nil {
			return fmt.Errorf(
				"capsule: invalid graph info identity: %w", err,
			)
		}

		if i > 0 && file.Path <= previousPath {
			return fmt.Errorf(
				"capsule: graph info files must have unique sorted paths",
			)
		}
		previousPath = file.Path

		version, exists := selectedVersions[file.Path]
		if !exists || version != file.Version {
			return fmt.Errorf(
				"capsule: graph info module absent from selected graph",
			)
		}

		if _, exists := fullVersions[file.Path]; exists {
			return fmt.Errorf(
				"capsule: graph info duplicates full module",
			)
		}

		expected := fmt.Sprintf("graph-info-%04d.info", i+1)
		if file.Artifact.Path != expected {
			return fmt.Errorf(
				"capsule: invalid graph info artifact path",
			)
		}

		if file.Artifact.Size < 0 ||
			file.Artifact.Size > MaxGoGraphInfoTotalSizeV3-total {
			return fmt.Errorf(
				"capsule: graph info size limit exceeded",
			)
		}
		total += file.Artifact.Size

		if !strings.HasPrefix(file.Artifact.SHA256, "sha256:") {
			return fmt.Errorf(
				"capsule: invalid graph info digest prefix",
			)
		}

		digest := strings.TrimPrefix(file.Artifact.SHA256, "sha256:")
		if len(digest) != 64 {
			return fmt.Errorf(
				"capsule: invalid graph info digest length",
			)
		}

		if _, err := hex.DecodeString(digest); err != nil {
			return fmt.Errorf(
				"capsule: invalid graph info SHA-256: %w", err,
			)
		}
	}

	return nil
}

// VerifyGoGraphInfoFileV3 verifies a bounded metadata-only
// .info artifact. It establishes capsule integrity and identity
// consistency, not external module provenance.
func VerifyGoGraphInfoFileV3(
	file GoGraphInfoFileV3,
	data []byte,
) error {
	if file.Artifact.Size < 0 ||
		file.Artifact.Size > MaxGoGraphInfoTotalSizeV3 ||
		int64(len(data)) != file.Artifact.Size {
		return fmt.Errorf("capsule: graph info size mismatch")
	}

	if SHA256(data) != file.Artifact.SHA256 {
		return fmt.Errorf("capsule: graph info SHA-256 mismatch")
	}

	var info struct {
		Version string `json:"Version"`
	}

	if err := json.Unmarshal(data, &info); err != nil {
		return fmt.Errorf("capsule: invalid graph info JSON: %w", err)
	}

	if info.Version != file.Version {
		return fmt.Errorf("capsule: graph info version mismatch")
	}

	return nil
}
