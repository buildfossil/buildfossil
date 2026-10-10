package capsule

import (
	"archive/tar"
	"bytes"
	"fmt"
	"path"
	"sort"
)

// BuildWorkspaceSnapshotV3 creates a Docker-compatible TAR archive
// from a previously verified schema-v3 capsule.
//
// It never reads workspace files from the host filesystem.
func BuildWorkspaceSnapshotV3(
	verified VerifiedCapsuleV3,
) ([]byte, error) {
	if verified.Manifest.SchemaVersion != SchemaVersionV3 {
		return nil, fmt.Errorf("capsule: snapshot requires schema v3")
	}

	if err := verified.Manifest.Validate(); err != nil {
		return nil, fmt.Errorf("capsule: invalid snapshot manifest: %w", err)
	}

	expected := make(map[string]FileMetadata, len(verified.Manifest.Workspace.Files))
	for _, metadata := range verified.Manifest.Workspace.Files {
		expected[metadata.Path] = metadata
	}

	if len(verified.Files) != len(expected) {
		return nil, fmt.Errorf("capsule: snapshot file count mismatch")
	}

	files := make([]WorkspaceFile, len(verified.Files))
	copy(files, verified.Files)

	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})

	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)

	// Track paths to reject duplicate workspace files.
	seen := make(map[string]struct{}, len(files))

	for _, file := range files {
		if _, exists := seen[file.Path]; exists {
			return nil, fmt.Errorf(
				"capsule: duplicate snapshot file %q",
				file.Path,
			)
		}
		seen[file.Path] = struct{}{}

		if err := ValidateWorkspacePath(file.Path, file.Mode); err != nil {
			return nil, fmt.Errorf("capsule: invalid snapshot path: %w", err)
		}

		metadata, ok := expected[file.Path]
		if !ok {
			return nil, fmt.Errorf("capsule: unexpected snapshot file %q", file.Path)
		}

		if err := VerifyWorkspaceFileV2(metadata, file.Data); err != nil {
			return nil, fmt.Errorf("capsule: invalid snapshot file %q: %w", file.Path, err)
		}

		header := &tar.Header{
			Name:     path.Clean(file.Path),
			Mode:     0644,
			Size:     int64(len(file.Data)),
			Typeflag: tar.TypeReg,
		}

		if err := writer.WriteHeader(header); err != nil {
			return nil, err
		}

		if _, err := writer.Write(file.Data); err != nil {
			return nil, err
		}
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	if int64(buffer.Len()) > MaxCapsuleSize {
		return nil, fmt.Errorf("capsule: snapshot exceeds size limit")
	}

	return buffer.Bytes(), nil
}
