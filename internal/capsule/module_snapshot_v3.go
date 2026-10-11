package capsule

import (
	"archive/tar"
	"bytes"
	"fmt"
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
	if verified.Manifest.SchemaVersion != SchemaVersionV3 {
		return nil, fmt.Errorf("capsule: module snapshot requires schema v3")
	}

	if err := verified.Manifest.Validate(); err != nil {
		return nil, fmt.Errorf("capsule: invalid manifest: %w", err)
	}

	if len(verified.Files) != len(verified.Manifest.Workspace.Files) {
		return nil, fmt.Errorf("capsule: workspace file count mismatch")
	}

	expected := make(map[string]FileMetadata)
	for _, metadata := range verified.Manifest.Workspace.Files {
		expected[metadata.Path] = metadata
	}

	seen := make(map[string]bool)
	var goSum []byte

	for _, file := range verified.Files {
		if seen[file.Path] {
			return nil, fmt.Errorf(
				"capsule: duplicate workspace file %q", file.Path,
			)
		}
		seen[file.Path] = true

		metadata, ok := expected[file.Path]
		if !ok {
			return nil, fmt.Errorf(
				"capsule: unexpected workspace file %q", file.Path,
			)
		}

		if err := ValidateWorkspacePath(file.Path, file.Mode); err != nil {
			return nil, err
		}

		if err := VerifyWorkspaceFileV2(metadata, file.Data); err != nil {
			return nil, err
		}

		if file.Path == "go.sum" {
			goSum = file.Data
		}
	}

	if !seen["go.sum"] {
		return nil, fmt.Errorf("capsule: missing go.sum")
	}

	selectedArtifacts, graphArtifacts, err := SplitGoArtifactsV3(
		verified.Manifest.GoModules,
		verified.Manifest.GoGraphModFiles,
		verified.Artifacts,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"capsule: split module artifacts: %w", err,
		)
	}

	if err := VerifyGoModuleArtifactSetV3(
		verified.Manifest.GoModules,
		selectedArtifacts,
		goSum,
	); err != nil {
		return nil, fmt.Errorf(
			"capsule: invalid module artifact set: %w", err,
		)
	}

	if err := VerifyGoGraphModArtifactSetV3(
		verified.Manifest.GoGraphModFiles,
		graphArtifacts,
		goSum,
	); err != nil {
		return nil, fmt.Errorf(
			"capsule: invalid graph module artifacts: %w", err,
		)
	}

	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)

	for index, mod := range verified.Manifest.GoModules {
		escapedPath, err := module.EscapePath(mod.Path)
		if err != nil {
			return nil, fmt.Errorf(
				"capsule: escape module path: %w", err,
			)
		}

		escapedVersion, err := module.EscapeVersion(mod.Version)
		if err != nil {
			return nil, fmt.Errorf(
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
				return nil, fmt.Errorf(
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
				return nil, fmt.Errorf(
					"capsule: write module TAR header: %w", err,
				)
			}

			if _, err := writer.Write(data); err != nil {
				return nil, fmt.Errorf(
					"capsule: write module TAR content: %w", err,
				)
			}
		}
	}

	for _, graphFile := range verified.Manifest.GoGraphModFiles {
		escapedPath, err := module.EscapePath(graphFile.Path)
		if err != nil {
			return nil, fmt.Errorf(
				"capsule: escape graph module path: %w", err,
			)
		}

		escapedVersion, err := module.EscapeVersion(graphFile.Version)
		if err != nil {
			return nil, fmt.Errorf(
				"capsule: escape graph module version: %w", err,
			)
		}

		name := graphFile.Artifact.Path
		data, ok := verified.Artifacts[name]
		if !ok {
			return nil, fmt.Errorf(
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
			return nil, fmt.Errorf(
				"capsule: write graph module TAR header: %w", err,
			)
		}

		if _, err := writer.Write(data); err != nil {
			return nil, fmt.Errorf(
				"capsule: write graph module TAR content: %w", err,
			)
		}
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("capsule: close module TAR: %w", err)
	}

	if int64(buffer.Len()) > MaxCapsuleSize {
		return nil, fmt.Errorf("capsule: module snapshot exceeds size limit")
	}

	return buffer.Bytes(), nil
}
