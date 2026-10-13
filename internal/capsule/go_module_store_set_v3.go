package capsule

import "fmt"

// verifyGoModuleSetFromStoreV3 verifies a selected-only store.
// Any extra artifact makes the store invalid.
func verifyGoModuleSetFromStoreV3(
	modules []GoModuleV3,
	goSum []byte,
	store *ArtifactStoreV3,
) error {
	if err := ValidateGoModulesV3(modules); err != nil {
		return err
	}

	if store == nil || store.closed {
		return fmt.Errorf("capsule: module artifact store unavailable")
	}

	expectedCount := len(modules) * MaxGoModuleArtifactsV3

	if len(store.names) != expectedCount {
		return fmt.Errorf("capsule: Go artifact set count mismatch")
	}

	expected := make(map[string]bool, expectedCount)

	for _, mod := range modules {
		for _, metadata := range mod.Artifacts {
			expected[metadata.Path] = true
		}
	}

	for name := range store.names {
		if !expected[name] {
			return fmt.Errorf(
				"capsule: unexpected Go artifact %q", name,
			)
		}
	}

	return verifyGoModuleSubsetFromStoreV3(
		modules, goSum, store,
	)
}

// verifyGoModuleSubsetFromStoreV3 verifies selected modules while
// allowing graph-only artifacts to exist in the same store.
//
// The caller must independently verify that every additional
// artifact is declared and valid.
func verifyGoModuleSubsetFromStoreV3(
	modules []GoModuleV3,
	goSum []byte,
	store *ArtifactStoreV3,
) error {
	if err := ValidateGoModulesV3(modules); err != nil {
		return err
	}

	if store == nil || store.closed {
		return fmt.Errorf("capsule: module artifact store unavailable")
	}

	seen := make(map[string]bool)
	var totalSize int64
	var unpackedTotal int64

	for index, mod := range modules {
		for _, metadata := range mod.Artifacts {
			if seen[metadata.Path] {
				return fmt.Errorf(
					"capsule: duplicate module artifact %q",
					metadata.Path,
				)
			}

			seen[metadata.Path] = true

			record, ok := store.names[metadata.Path]
			if !ok {
				return fmt.Errorf(
					"capsule: missing Go artifact %q",
					metadata.Path,
				)
			}

			if record.size < 0 ||
				record.size > MaxGoModuleTotalSizeV3-totalSize {
				return fmt.Errorf(
					"capsule: Go artifact size limit exceeded",
				)
			}

			totalSize += record.size
		}

		unpackedSize, err := verifyGoModuleFromStoreMeasuredV3(
			mod, index+1, goSum, store,
		)
		if err != nil {
			return fmt.Errorf(
				"capsule: verify Go module %d (%s): %w",
				index+1, mod.Path, err,
			)
		}

		if unpackedSize < 0 ||
			unpackedSize > MaxGoModuleUnpackedTotalSizeV3-unpackedTotal {
			return fmt.Errorf(
				"capsule: total Go module ZIP unpacked size exceeded",
			)
		}
		unpackedTotal += unpackedSize

	}

	return nil
}
