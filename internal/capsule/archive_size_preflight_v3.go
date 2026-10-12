package capsule

import "fmt"

// validateArchiveSizePreflightV3 checks the complete canonical TAR
// size using metadata, without allocating artifact content.
//
// Callers validate manifest and artifacts separately before using it.
func validateArchiveSizePreflightV3(
	manifest Manifest,
	manifestSize int64,
) error {
	if manifestSize < 0 || manifestSize > MaxManifestSize {
		return fmt.Errorf("capsule: invalid manifest size")
	}

	for _, file := range manifest.Workspace.Files {
		if file.Size < 0 {
			return fmt.Errorf("capsule: invalid workspace size")
		}
	}

	expected := expectedTarSize(manifestSize, manifest.Workspace)
	if expected < 0 || expected > MaxCapsuleSizeV3 {
		return fmt.Errorf("capsule: v3 archive exceeds size limit")
	}

	add := func(size int64) error {
		if size < 0 || size > MaxCapsuleSizeV3-expected {
			return fmt.Errorf("capsule: v3 archive exceeds size limit")
		}

		entry := tarEntrySize(size)
		if entry > MaxCapsuleSizeV3-expected {
			return fmt.Errorf("capsule: v3 archive exceeds size limit")
		}

		expected += entry
		return nil
	}

	for _, mod := range manifest.GoModules {
		for _, artifact := range mod.Artifacts {
			if err := add(artifact.Size); err != nil {
				return err
			}
		}
	}

	for _, graph := range manifest.GoGraphModFiles {
		if err := add(graph.Artifact.Size); err != nil {
			return err
		}
	}

	for _, graph := range manifest.GoGraphInfoFiles {
		if err := add(graph.Artifact.Size); err != nil {
			return err
		}
	}

	return nil
}
