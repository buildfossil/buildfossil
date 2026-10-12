package capsule

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"path"
	"sort"
)

// BuildWorkspaceSnapshotV3 preserves the existing in-memory API.
func BuildWorkspaceSnapshotV3(
	verified VerifiedCapsuleV3,
) ([]byte, error) {
	var output bytes.Buffer
	if err := WriteWorkspaceSnapshotV3(&output, verified); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

// WriteWorkspaceSnapshotV3 writes a verified workspace TAR to w.
// On write failure, w may contain a partial archive.
// The caller must not publish or execute a partial archive.
func WriteWorkspaceSnapshotV3(
	w io.Writer,
	verified VerifiedCapsuleV3,
) error {
	if w == nil {
		return fmt.Errorf("capsule: nil snapshot writer")
	}
	if verified.Manifest.SchemaVersion != SchemaVersionV3 {
		return fmt.Errorf("capsule: snapshot requires schema v3")
	}
	if err := verified.Manifest.Validate(); err != nil {
		return fmt.Errorf("capsule: invalid snapshot manifest: %w", err)
	}

	expected := make(
		map[string]FileMetadata,
		len(verified.Manifest.Workspace.Files),
	)
	for _, metadata := range verified.Manifest.Workspace.Files {
		expected[metadata.Path] = metadata
	}
	if len(verified.Files) != len(expected) {
		return fmt.Errorf("capsule: snapshot file count mismatch")
	}

	files := make([]WorkspaceFile, len(verified.Files))
	copy(files, verified.Files)
	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})

	// Validate every file before emitting any bytes.
	seen := make(map[string]struct{}, len(files))
	var estimated int64 = 1024 // TAR end-of-archive blocks

	for _, file := range files {
		if _, exists := seen[file.Path]; exists {
			return fmt.Errorf("capsule: duplicate snapshot file %q", file.Path)
		}
		seen[file.Path] = struct{}{}

		if err := ValidateWorkspacePath(file.Path, file.Mode); err != nil {
			return fmt.Errorf("capsule: invalid snapshot path: %w", err)
		}

		metadata, ok := expected[file.Path]
		if !ok {
			return fmt.Errorf("capsule: unexpected snapshot file %q", file.Path)
		}
		if err := VerifyWorkspaceFileV2(metadata, file.Data); err != nil {
			return fmt.Errorf("capsule: invalid snapshot file %q: %w", file.Path, err)
		}

		// Each simple regular TAR entry uses a 512-byte header
		// followed by content padded to a 512-byte boundary.
		size := int64(len(file.Data))
		estimated += 512 + ((size + 511) / 512 * 512)
		if estimated > MaxCapsuleSizeV3 {
			return fmt.Errorf("capsule: snapshot exceeds size limit")
		}
	}

	limited := &workspaceSnapshotLimitWriterV3{
		dst:       w,
		remaining: MaxCapsuleSizeV3,
	}
	writer := tar.NewWriter(limited)

	for _, file := range files {
		header := &tar.Header{
			Name:     path.Clean(file.Path),
			Mode:     0644,
			Size:     int64(len(file.Data)),
			Typeflag: tar.TypeReg,
		}
		if err := writer.WriteHeader(header); err != nil {
			return err
		}
		if _, err := writer.Write(file.Data); err != nil {
			return err
		}
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return nil
}

type workspaceSnapshotLimitWriterV3 struct {
	dst       io.Writer
	remaining int64
}

func (w *workspaceSnapshotLimitWriterV3) Write(data []byte) (int, error) {
	if int64(len(data)) > w.remaining {
		return 0, fmt.Errorf("capsule: snapshot exceeds size limit")
	}
	n, err := w.dst.Write(data)
	if n < 0 || n > len(data) {
		return 0, fmt.Errorf("capsule: invalid snapshot writer count")
	}
	w.remaining -= int64(n)
	if err == nil && n != len(data) {
		return n, io.ErrShortWrite
	}
	return n, err
}
