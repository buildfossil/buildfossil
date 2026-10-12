package capsule

import (
	"bytes"
	"fmt"
	"strings"
)

// AddGoGraphArtifactsToStoreV3 appends validated graph-only .mod and
// .info artifacts to a store that may already contain full Go modules.
//
// Graph metadata is deliberately bounded to its existing 1 MiB
// category budgets. The caller owns the store and closes it.
func AddGoGraphArtifactsToStoreV3(
	manifest Manifest,
	goSum []byte,
	graphMods map[string][]byte,
	graphInfos map[string][]byte,
	store *ArtifactStoreV3,
) error {
	if store == nil || store.closed {
		return fmt.Errorf("capsule: unavailable artifact store")
	}

	if err := ValidateGoGraphModFilesV3(
		manifest.GoGraphModFiles,
	); err != nil {
		return err
	}

	if err := ValidateGoGraphInfoFilesV3(
		manifest.GoGraphInfoFiles,
		EffectiveGoSelectedModulesV3(manifest),
		manifest.GoModules,
	); err != nil {
		return err
	}

	if err := VerifyGoGraphModArtifactSetV3(
		manifest.GoGraphModFiles, graphMods, goSum,
	); err != nil {
		return fmt.Errorf("capsule: invalid graph mod set: %w", err)
	}

	if err := VerifyGoGraphInfoArtifactSetV3(
		manifest.GoGraphInfoFiles, graphInfos,
	); err != nil {
		return fmt.Errorf("capsule: invalid graph info set: %w", err)
	}

	// Preflight collisions so duplicate names cannot cause a
	// partially appended graph set.
	expected := make(map[string]bool)

	check := func(metadata GoModuleArtifact) error {
		if expected[metadata.Path] {
			return fmt.Errorf(
				"capsule: duplicate graph name %q", metadata.Path,
			)
		}
		expected[metadata.Path] = true

		if _, exists := store.names[metadata.Path]; exists {
			return fmt.Errorf(
				"capsule: graph artifact collides with stored file %q",
				metadata.Path,
			)
		}
		return nil
	}

	for _, graph := range manifest.GoGraphModFiles {
		if err := check(graph.Artifact); err != nil {
			return err
		}
	}
	for _, graph := range manifest.GoGraphInfoFiles {
		if err := check(graph.Artifact); err != nil {
			return err
		}
	}

	add := func(metadata GoModuleArtifact, data []byte) error {
		if err := store.Put(
			metadata.Path,
			bytes.NewReader(data),
			metadata.Size,
			strings.TrimPrefix(metadata.SHA256, "sha256:"),
		); err != nil {
			return fmt.Errorf(
				"capsule: store graph artifact %q: %w",
				metadata.Path, err,
			)
		}
		return nil
	}

	for _, graph := range manifest.GoGraphModFiles {
		if err := add(
			graph.Artifact,
			graphMods[graph.Artifact.Path],
		); err != nil {
			return err
		}
	}

	for _, graph := range manifest.GoGraphInfoFiles {
		if err := add(
			graph.Artifact,
			graphInfos[graph.Artifact.Path],
		); err != nil {
			return err
		}
	}

	return nil
}
