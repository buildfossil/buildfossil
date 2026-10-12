package capsule

import "fmt"

// verifyCompleteGoArtifactStoreV3 validates the full artifact set
// declared by a previously validated schema-v3 manifest.
//
// The caller must validate the manifest before calling this function.
// The store must not be concurrently modified.
func verifyCompleteGoArtifactStoreV3(
	manifest Manifest,
	goSum []byte,
	store *ArtifactStoreV3,
) error {
	if manifest.SchemaVersion != SchemaVersionV3 {
		return fmt.Errorf("capsule: expected schema v3")
	}

	if err := manifest.Validate(); err != nil {
		return fmt.Errorf("capsule: invalid manifest: %w", err)
	}

	if store == nil || store.closed {
		return fmt.Errorf("capsule: artifact store unavailable")
	}

	expected := make(map[string]bool)

	add := func(name string) error {
		if expected[name] {
			return fmt.Errorf(
				"capsule: duplicate declared artifact %q", name,
			)
		}
		expected[name] = true
		return nil
	}

	for _, mod := range manifest.GoModules {
		for _, artifact := range mod.Artifacts {
			if err := add(artifact.Path); err != nil {
				return err
			}
		}
	}

	for _, graph := range manifest.GoGraphModFiles {
		if err := add(graph.Artifact.Path); err != nil {
			return err
		}
	}

	for _, info := range manifest.GoGraphInfoFiles {
		if err := add(info.Artifact.Path); err != nil {
			return err
		}
	}

	if len(store.names) != len(expected) {
		return fmt.Errorf(
			"capsule: artifact store count mismatch: expected %d, got %d",
			len(expected), len(store.names),
		)
	}

	for name := range store.names {
		if !expected[name] {
			return fmt.Errorf(
				"capsule: unexpected stored artifact %q", name,
			)
		}
	}

	for name := range expected {
		if _, ok := store.names[name]; !ok {
			return fmt.Errorf(
				"capsule: missing stored artifact %q", name,
			)
		}
	}

	if err := verifyGoModuleSubsetFromStoreV3(
		manifest.GoModules, goSum, store,
	); err != nil {
		return fmt.Errorf(
			"capsule: selected module verification: %w", err,
		)
	}

	if err := verifyGoGraphArtifactsFromStoreV3(
		manifest, goSum, store,
	); err != nil {
		return fmt.Errorf(
			"capsule: graph artifact verification: %w", err,
		)
	}

	return nil
}
