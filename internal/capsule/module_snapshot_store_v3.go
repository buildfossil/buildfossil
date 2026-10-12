package capsule

import (
	"archive/tar"
	"fmt"
	"io"
	"path"

	"golang.org/x/mod/module"
)

// WriteGoModuleSnapshotFromStoreV3 streams a verified module-cache
// snapshot from disk-backed artifacts without loading ZIPs into RAM.
//
// The caller must discard the destination if an error occurs.
func WriteGoModuleSnapshotFromStoreV3(
	w io.Writer,
	verified VerifiedStoredCapsuleV3,
) error {
	if w == nil {
		return fmt.Errorf("capsule: nil module snapshot writer")
	}

	manifest := verified.Manifest

	if manifest.SchemaVersion != SchemaVersionV3 {
		return fmt.Errorf("capsule: module snapshot requires schema v3")
	}

	if err := manifest.Validate(); err != nil {
		return fmt.Errorf("capsule: invalid manifest: %w", err)
	}

	if len(verified.Files) != len(manifest.Workspace.Files) {
		return fmt.Errorf("capsule: workspace file count mismatch")
	}

	expected := make(
		map[string]FileMetadata,
		len(manifest.Workspace.Files),
	)
	for _, metadata := range manifest.Workspace.Files {
		expected[metadata.Path] = metadata
	}

	seen := make(map[string]bool)
	var goSum []byte
	var goMod []byte

	for _, file := range verified.Files {
		if seen[file.Path] {
			return fmt.Errorf(
				"capsule: duplicate workspace file %q", file.Path,
			)
		}
		seen[file.Path] = true

		metadata, ok := expected[file.Path]
		if !ok {
			return fmt.Errorf(
				"capsule: unexpected workspace file %q", file.Path,
			)
		}

		if err := ValidateWorkspacePath(
			file.Path, file.Mode,
		); err != nil {
			return err
		}

		if err := VerifyWorkspaceFileV2(
			metadata, file.Data,
		); err != nil {
			return err
		}

		switch file.Path {
		case "go.sum":
			goSum = file.Data
		case "go.mod":
			goMod = file.Data
		}
	}

	if !seen["go.sum"] || !seen["go.mod"] {
		return fmt.Errorf("capsule: missing workspace go.mod or go.sum")
	}

	if err := ValidateGoSelectedModuleConsistencyV3(
		goMod, manifest,
	); err != nil {
		return fmt.Errorf(
			"capsule: invalid Go module declaration: %w", err,
		)
	}

	// Verify all dependencies before producing any TAR bytes.
	if err := verifyCompleteGoArtifactStoreV3(
		manifest, goSum, verified.Store,
	); err != nil {
		return fmt.Errorf(
			"capsule: verify stored dependencies: %w", err,
		)
	}

	limited := &workspaceSnapshotLimitWriterV3{
		dst:       w,
		remaining: MaxCapsuleSizeV3,
	}
	writer := tar.NewWriter(limited)

	writeArtifact := func(
		source string,
		destination string,
		expectedSize int64,
	) error {
		file, err := verified.Store.Open(source)
		if err != nil {
			return fmt.Errorf(
				"capsule: open module snapshot artifact %q: %w",
				source, err,
			)
		}

		header := &tar.Header{
			Name:     destination,
			Mode:     0644,
			Size:     expectedSize,
			Typeflag: tar.TypeReg,
		}

		if err := writer.WriteHeader(header); err != nil {
			_ = file.Close()
			return fmt.Errorf(
				"capsule: write module TAR header: %w", err,
			)
		}

		written, copyErr := io.CopyN(
			writer, file, expectedSize,
		)
		closeErr := file.Close()

		if copyErr != nil {
			return fmt.Errorf(
				"capsule: write artifact %q: %w",
				source, copyErr,
			)
		}
		if written != expectedSize {
			return fmt.Errorf(
				"capsule: short artifact write %q", source,
			)
		}
		if closeErr != nil {
			return fmt.Errorf(
				"capsule: close artifact %q: %w",
				source, closeErr,
			)
		}
		return nil
	}

	for index, mod := range manifest.GoModules {
		escapedPath, err := module.EscapePath(mod.Path)
		if err != nil {
			return fmt.Errorf(
				"capsule: escape module path: %w", err,
			)
		}

		escapedVersion, err := module.EscapeVersion(mod.Version)
		if err != nil {
			return fmt.Errorf(
				"capsule: escape module version: %w", err,
			)
		}

		directory := path.Join(
			"cache", "download", escapedPath, "@v",
		)

		metadataByName := make(
			map[string]GoModuleArtifact,
			len(mod.Artifacts),
		)
		for _, artifact := range mod.Artifacts {
			metadataByName[artifact.Path] = artifact
		}

		for _, ext := range []string{
			".zip", ".mod", ".info",
		} {
			name := fmt.Sprintf(
				"module-%04d%s", index+1, ext,
			)

			metadata, ok := metadataByName[name]
			if !ok {
				return fmt.Errorf(
					"capsule: missing module metadata %q", name,
				)
			}

			if err := writeArtifact(
				name,
				path.Join(directory, escapedVersion+ext),
				metadata.Size,
			); err != nil {
				return err
			}
		}
	}

	for _, graph := range manifest.GoGraphModFiles {
		escapedPath, err := module.EscapePath(graph.Path)
		if err != nil {
			return err
		}
		escapedVersion, err := module.EscapeVersion(graph.Version)
		if err != nil {
			return err
		}

		if err := writeArtifact(
			graph.Artifact.Path,
			path.Join(
				"cache", "download", escapedPath,
				"@v", escapedVersion+".mod",
			),
			graph.Artifact.Size,
		); err != nil {
			return err
		}
	}

	for _, graph := range manifest.GoGraphInfoFiles {
		escapedPath, err := module.EscapePath(graph.Path)
		if err != nil {
			return err
		}
		escapedVersion, err := module.EscapeVersion(graph.Version)
		if err != nil {
			return err
		}

		if err := writeArtifact(
			graph.Artifact.Path,
			path.Join(
				"cache", "download", escapedPath,
				"@v", escapedVersion+".info",
			),
			graph.Artifact.Size,
		); err != nil {
			return err
		}
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf(
			"capsule: close module TAR: %w", err,
		)
	}

	return nil
}
