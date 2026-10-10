package capsule

import (
	"archive/tar"
	"bytes"
	"fmt"
	"path"

	"golang.org/x/mod/module"
)

// BuildGoModuleSnapshotV3 creates a TAR containing the
// verified Go module download cache.
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

	if len(verified.Manifest.GoModules) > MaxGoModulesV3 {
		return nil, fmt.Errorf("capsule: too many Go modules")
	}

	if len(verified.Manifest.GoModules) == 0 && len(verified.Artifacts) != 0 {
		return nil, fmt.Errorf("capsule: unexpected artifacts without Go modules")
	}

	if len(verified.Files) != len(verified.Manifest.Workspace.Files) {
		return nil, fmt.Errorf("capsule: workspace file count mismatch")
	}

	// Validate workspace contents and locate the verified go.sum.
	// Do not rely on ordering in the supplied Files slice.
	expected := make(map[string]FileMetadata)
	for _, metadata := range verified.Manifest.Workspace.Files {
		expected[metadata.Path] = metadata
	}

	seen := make(map[string]bool)
	var goSum []byte

	for _, file := range verified.Files {
		if seen[file.Path] {
			return nil, fmt.Errorf("capsule: duplicate workspace file %q", file.Path)
		}
		seen[file.Path] = true

		metadata, ok := expected[file.Path]
		if !ok {
			return nil, fmt.Errorf("capsule: unexpected workspace file %q", file.Path)
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

	if len(verified.Manifest.GoModules) == 0 {
		var buffer bytes.Buffer
		writer := tar.NewWriter(&buffer)

		if err := writer.Close(); err != nil {
			return nil, fmt.Errorf("capsule: close empty module TAR: %w", err)
		}

		return buffer.Bytes(), nil
	}

	mod := verified.Manifest.GoModules[0]

	if err := VerifyGoModuleArtifactsV3(
		mod,
		verified.Artifacts,
		goSum,
	); err != nil {
		return nil, fmt.Errorf("capsule: invalid module artifacts: %w", err)
	}

	escapedPath, err := module.EscapePath(mod.Path)
	if err != nil {
		return nil, fmt.Errorf("capsule: escape module path: %w", err)
	}

	escapedVersion, err := module.EscapeVersion(mod.Version)
	if err != nil {
		return nil, fmt.Errorf("capsule: escape module version: %w", err)
	}

	directory := path.Join(
		"cache",
		"download",
		escapedPath,
		"@v",
	)

	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)

	// Fixed order ensures deterministic snapshots.
	for _, ext := range []string{".zip", ".mod", ".info"} {
		source := "module-0001" + ext
		data := verified.Artifacts[source]

		header := &tar.Header{
			Name:     path.Join(directory, escapedVersion+ext),
			Mode:     0644,
			Size:     int64(len(data)),
			Typeflag: tar.TypeReg,
		}

		if err := writer.WriteHeader(header); err != nil {
			return nil, fmt.Errorf("capsule: write module TAR header: %w", err)
		}

		if _, err := writer.Write(data); err != nil {
			return nil, fmt.Errorf("capsule: write module TAR content: %w", err)
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
