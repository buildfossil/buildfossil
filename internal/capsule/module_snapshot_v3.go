package capsule

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"path"

	"golang.org/x/mod/module"
)

// BuildGoModuleSnapshotV3 creates a deterministic TAR containing
// the verified Go module download cache.
//
// It does not read artifacts from host staging.
func BuildGoModuleSnapshotV3(
	verified VerifiedCapsuleV3,
) ([]byte, error) {
	var buffer bytes.Buffer
	if err := WriteGoModuleSnapshotV3(&buffer, verified); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// WriteGoModuleSnapshotV3 writes a verified module-cache TAR to w.
// On error the destination may contain an incomplete archive;
// callers must discard it rather than pass it to Docker.
func WriteGoModuleSnapshotV3(
	w io.Writer,
	verified VerifiedCapsuleV3,
) error {
	if w == nil {
		return fmt.Errorf("capsule: nil module snapshot writer")
	}
	if verified.Manifest.SchemaVersion != SchemaVersionV3 {
		return fmt.Errorf("capsule: module snapshot requires schema v3")
	}

	if err := verified.Manifest.Validate(); err != nil {
		return fmt.Errorf("capsule: invalid manifest: %w", err)
	}

	if len(verified.Files) != len(verified.Manifest.Workspace.Files) {
		return fmt.Errorf("capsule: workspace file count mismatch")
	}

	expected := make(map[string]FileMetadata)
	for _, metadata := range verified.Manifest.Workspace.Files {
		expected[metadata.Path] = metadata
	}

	seen := make(map[string]bool)
	var goSum []byte

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

		if err := ValidateWorkspacePath(file.Path, file.Mode); err != nil {
			return err
		}

		if err := VerifyWorkspaceFileV2(metadata, file.Data); err != nil {
			return err
		}

		if file.Path == "go.sum" {
			goSum = file.Data
		}
	}

	if !seen["go.sum"] {
		return fmt.Errorf("capsule: missing go.sum")
	}

	selectedArtifacts, graphArtifacts, infoArtifacts, err :=
		SplitExtendedGoArtifactsV3(
			verified.Manifest,
			verified.Artifacts,
		)
	if err != nil {
		return fmt.Errorf(
			"capsule: split module artifacts: %w", err,
		)
	}

	if err := VerifyGoModuleArtifactSetV3(
		verified.Manifest.GoModules,
		selectedArtifacts,
		goSum,
	); err != nil {
		return fmt.Errorf(
			"capsule: invalid module artifact set: %w", err,
		)
	}

	if err := VerifyGoGraphModArtifactSetV3(
		verified.Manifest.GoGraphModFiles,
		graphArtifacts,
		goSum,
	); err != nil {
		return fmt.Errorf(
			"capsule: invalid graph module artifacts: %w", err,
		)
	}

	if err := VerifyGoGraphInfoArtifactSetV3(
		verified.Manifest.GoGraphInfoFiles,
		infoArtifacts,
	); err != nil {
		return fmt.Errorf(
			"capsule: invalid graph info artifacts: %w", err,
		)
	}

	limited := &workspaceSnapshotLimitWriterV3{
		dst:       w,
		remaining: MaxCapsuleSizeV3,
	}
	writer := tar.NewWriter(limited)

	for index, mod := range verified.Manifest.GoModules {
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

		for _, ext := range []string{".zip", ".mod", ".info"} {
			source := fmt.Sprintf("module-%04d%s", index+1, ext)

			data, ok := verified.Artifacts[source]
			if !ok {
				return fmt.Errorf(
					"capsule: missing module artifact %q", source,
				)
			}

			header := &tar.Header{
				Name:     path.Join(directory, escapedVersion+ext),
				Mode:     0644,
				Size:     int64(len(data)),
				Typeflag: tar.TypeReg,
			}

			if err := writer.WriteHeader(header); err != nil {
				return fmt.Errorf(
					"capsule: write module TAR header: %w", err,
				)
			}

			if _, err := writer.Write(data); err != nil {
				return fmt.Errorf(
					"capsule: write module TAR content: %w", err,
				)
			}
		}
	}

	for _, graphFile := range verified.Manifest.GoGraphModFiles {
		escapedPath, err := module.EscapePath(graphFile.Path)
		if err != nil {
			return fmt.Errorf(
				"capsule: escape graph module path: %w", err,
			)
		}

		escapedVersion, err := module.EscapeVersion(graphFile.Version)
		if err != nil {
			return fmt.Errorf(
				"capsule: escape graph module version: %w", err,
			)
		}

		name := graphFile.Artifact.Path
		data, ok := verified.Artifacts[name]
		if !ok {
			return fmt.Errorf(
				"capsule: missing graph module artifact %q", name,
			)
		}

		header := &tar.Header{
			Name: path.Join(
				"cache", "download",
				escapedPath, "@v",
				escapedVersion+".mod",
			),
			Mode:     0644,
			Size:     int64(len(data)),
			Typeflag: tar.TypeReg,
		}

		if err := writer.WriteHeader(header); err != nil {
			return fmt.Errorf(
				"capsule: write graph module TAR header: %w", err,
			)
		}

		if _, err := writer.Write(data); err != nil {
			return fmt.Errorf(
				"capsule: write graph module TAR content: %w", err,
			)
		}
	}

	for _, infoFile := range verified.Manifest.GoGraphInfoFiles {
		escapedPath, err := module.EscapePath(infoFile.Path)
		if err != nil {
			return fmt.Errorf(
				"capsule: escape graph info module path: %w", err,
			)
		}

		escapedVersion, err := module.EscapeVersion(infoFile.Version)
		if err != nil {
			return fmt.Errorf(
				"capsule: escape graph info version: %w", err,
			)
		}

		data, ok := infoArtifacts[infoFile.Artifact.Path]
		if !ok {
			return fmt.Errorf(
				"capsule: missing graph info artifact %q",
				infoFile.Artifact.Path,
			)
		}

		header := &tar.Header{
			Name: path.Join(
				"cache", "download",
				escapedPath, "@v",
				escapedVersion+".info",
			),
			Mode:     0644,
			Size:     int64(len(data)),
			Typeflag: tar.TypeReg,
		}

		if err := writer.WriteHeader(header); err != nil {
			return fmt.Errorf(
				"capsule: write graph info TAR header: %w", err,
			)
		}

		if _, err := writer.Write(data); err != nil {
			return fmt.Errorf(
				"capsule: write graph info TAR data: %w", err,
			)
		}
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("capsule: close module TAR: %w", err)
	}

	return nil
}
