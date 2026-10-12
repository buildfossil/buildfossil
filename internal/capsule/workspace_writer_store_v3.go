package capsule

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"io"
)

// WriteWorkspaceFromStoreV3 emits a schema-v3 capsule using
// verified disk-backed dependency artifacts.
//
// All validation completes before output begins. A destination
// write failure can leave a partial TAR; callers must discard it.
func WriteWorkspaceFromStoreV3(
	w io.Writer,
	manifest Manifest,
	files []WorkspaceFile,
	store *ArtifactStoreV3,
) error {
	if w == nil {
		return fmt.Errorf("capsule: nil v3 writer")
	}
	if manifest.SchemaVersion != SchemaVersionV3 {
		return fmt.Errorf("capsule: v3 writer requires schema version 3")
	}
	if len(manifest.GoModules) > MaxGoModulesV3 {
		return fmt.Errorf("capsule: too many Go modules")
	}

	metadata := make([]FileMetadata, 0, len(files))

	var goMod, goSum []byte
	foundMod, foundSum := false, false

	for _, file := range files {
		if err := ValidateWorkspacePath(file.Path, file.Mode); err != nil {
			return fmt.Errorf("capsule: invalid workspace file: %w", err)
		}
		if int64(len(file.Data)) > MaxWorkspaceFileSize {
			return fmt.Errorf("capsule: workspace file too large: %q", file.Path)
		}

		metadata = append(metadata, FileMetadata{
			Path:   file.Path,
			Size:   int64(len(file.Data)),
			SHA256: SHA256(file.Data),
		})

		switch file.Path {
		case "go.mod":
			if foundMod {
				return fmt.Errorf("capsule: duplicate workspace go.mod")
			}
			foundMod = true
			goMod = file.Data
		case "go.sum":
			if foundSum {
				return fmt.Errorf("capsule: duplicate workspace go.sum")
			}
			foundSum = true
			goSum = file.Data
		}
	}

	if !foundMod || !foundSum {
		return fmt.Errorf("capsule: v3 requires workspace/go.mod and go.sum")
	}

	if err := ValidateGoSelectedModuleConsistencyV3(
		goMod, manifest,
	); err != nil {
		return fmt.Errorf(
			"capsule: invalid Go dependency declaration: %w", err,
		)
	}

	manifest.Workspace = Workspace{Files: metadata}
	if err := manifest.Validate(); err != nil {
		return fmt.Errorf("capsule: invalid v3 manifest: %w", err)
	}

	// Reject missing, extra, or corrupted dependencies before writing.
	if err := verifyCompleteGoArtifactStoreV3(
		manifest, goSum, store,
	); err != nil {
		return fmt.Errorf("capsule: verify stored artifacts: %w", err)
	}

	manifestData, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("capsule: encode manifest: %w", err)
	}
	if len(manifestData) > MaxManifestSize {
		return fmt.Errorf("capsule: manifest too large")
	}

	if err := validateArchiveSizePreflightV3(
		manifest, int64(len(manifestData)),
	); err != nil {
		return err
	}

	limited := &workspaceSnapshotLimitWriterV3{
		dst: w, remaining: MaxCapsuleSizeV3,
	}
	tw := tar.NewWriter(limited)

	writeBytes := func(name string, data []byte, mode int64) error {
		header := &tar.Header{
			Name:     name,
			Mode:     mode,
			Size:     int64(len(data)),
			Typeflag: tar.TypeReg,
		}
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		_, err := tw.Write(data)
		return err
	}

	writeArtifact := func(artifact GoModuleArtifact) error {
		file, err := store.Open(artifact.Path)
		if err != nil {
			return fmt.Errorf(
				"capsule: open dependency %q: %w", artifact.Path, err,
			)
		}

		header := &tar.Header{
			Name:     "dependencies/" + artifact.Path,
			Mode:     0600,
			Size:     artifact.Size,
			Typeflag: tar.TypeReg,
		}
		if err := tw.WriteHeader(header); err != nil {
			_ = file.Close()
			return err
		}

		written, copyErr := io.CopyN(tw, file, artifact.Size)
		closeErr := file.Close()
		if copyErr != nil {
			return fmt.Errorf(
				"capsule: copy dependency %q: %w",
				artifact.Path, copyErr,
			)
		}
		if written != artifact.Size {
			return fmt.Errorf(
				"capsule: short dependency write %q", artifact.Path,
			)
		}
		if closeErr != nil {
			return fmt.Errorf(
				"capsule: close dependency %q: %w",
				artifact.Path, closeErr,
			)
		}
		return nil
	}

	if err := writeBytes("manifest.json", manifestData, 0600); err != nil {
		return fmt.Errorf("capsule: write manifest: %w", err)
	}

	for _, file := range files {
		if err := writeBytes(
			"workspace/"+file.Path, file.Data,
			int64(file.Mode.Perm()),
		); err != nil {
			return fmt.Errorf(
				"capsule: write workspace file %q: %w",
				file.Path, err,
			)
		}
	}

	for _, mod := range manifest.GoModules {
		for _, artifact := range mod.Artifacts {
			if err := writeArtifact(artifact); err != nil {
				return err
			}
		}
	}
	for _, graph := range manifest.GoGraphModFiles {
		if err := writeArtifact(graph.Artifact); err != nil {
			return err
		}
	}
	for _, graph := range manifest.GoGraphInfoFiles {
		if err := writeArtifact(graph.Artifact); err != nil {
			return err
		}
	}

	if err := tw.Close(); err != nil {
		return fmt.Errorf("capsule: finalize v3 archive: %w", err)
	}

	return nil
}
