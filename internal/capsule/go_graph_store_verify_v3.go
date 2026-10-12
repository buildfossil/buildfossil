package capsule

import (
	"fmt"
	"io"
	"strings"
)

// readGraphArtifactFromStoreV3 validates the declared metadata and
// reads a bounded, SHA-256-verified graph artifact from the store.
func readGraphArtifactFromStoreV3(
	store *ArtifactStoreV3,
	metadata GoModuleArtifact,
	maxSize int64,
) ([]byte, error) {
	if store == nil || store.closed {
		return nil, fmt.Errorf("capsule: graph artifact store unavailable")
	}

	if metadata.Size < 0 || metadata.Size > maxSize {
		return nil, fmt.Errorf("capsule: graph artifact size limit exceeded")
	}

	if !strings.HasPrefix(metadata.SHA256, "sha256:") ||
		len(metadata.SHA256) != len("sha256:")+64 {
		return nil, fmt.Errorf("capsule: invalid graph artifact checksum")
	}

	record, ok := store.names[metadata.Path]
	if !ok ||
		record.size != metadata.Size ||
		record.digest != strings.TrimPrefix(metadata.SHA256, "sha256:") {
		return nil, fmt.Errorf(
			"capsule: graph artifact metadata mismatch %q", metadata.Path,
		)
	}

	file, err := store.Open(metadata.Path)
	if err != nil {
		return nil, fmt.Errorf(
			"capsule: open verified graph artifact %q: %w",
			metadata.Path, err,
		)
	}

	data, readErr := io.ReadAll(io.LimitReader(file, metadata.Size+1))
	closeErr := file.Close()

	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if int64(len(data)) != metadata.Size {
		return nil, fmt.Errorf(
			"capsule: graph artifact size mismatch %q", metadata.Path,
		)
	}

	return data, nil
}

// verifyGoGraphArtifactsFromStoreV3 checks graph-only .mod and
// metadata-only .info files. Selected ZIP modules are checked
// separately by verifyGoModuleSubsetFromStoreV3.
//
// The caller must validate the complete manifest and ensure the
// store contains no undeclared artifacts.
func verifyGoGraphArtifactsFromStoreV3(
	manifest Manifest,
	goSum []byte,
	store *ArtifactStoreV3,
) error {
	if store == nil || store.closed {
		return fmt.Errorf("capsule: graph artifact store unavailable")
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

	var totalModSize int64
	var totalInfoSize int64
	seen := make(map[string]bool)

	for _, graphFile := range manifest.GoGraphModFiles {
		metadata := graphFile.Artifact

		if seen[metadata.Path] {
			return fmt.Errorf(
				"capsule: duplicate graph artifact %q", metadata.Path,
			)
		}
		seen[metadata.Path] = true

		remaining := MaxGoGraphModTotalSizeV3 - totalModSize
		data, err := readGraphArtifactFromStoreV3(
			store, metadata, remaining,
		)
		if err != nil {
			return err
		}

		if err := VerifyGoGraphModFileV3(
			graphFile, data, goSum,
		); err != nil {
			return fmt.Errorf(
				"capsule: verify graph module %s@%s: %w",
				graphFile.Path, graphFile.Version, err,
			)
		}

		totalModSize += metadata.Size
	}

	for _, graphFile := range manifest.GoGraphInfoFiles {
		metadata := graphFile.Artifact

		if seen[metadata.Path] {
			return fmt.Errorf(
				"capsule: duplicate graph artifact %q", metadata.Path,
			)
		}
		seen[metadata.Path] = true

		remaining := MaxGoGraphInfoTotalSizeV3 - totalInfoSize
		data, err := readGraphArtifactFromStoreV3(
			store, metadata, remaining,
		)
		if err != nil {
			return err
		}

		if err := VerifyGoGraphInfoFileV3(
			graphFile, data,
		); err != nil {
			return fmt.Errorf(
				"capsule: verify graph info %s@%s: %w",
				graphFile.Path, graphFile.Version, err,
			)
		}

		totalInfoSize += metadata.Size
	}

	return nil
}
