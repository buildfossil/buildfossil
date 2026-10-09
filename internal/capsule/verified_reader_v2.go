package capsule

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

type VerifiedCapsuleV2 struct {
	Manifest Manifest
	Files    []WorkspaceFile
}

func ReadVerifiedV2(filename string) (VerifiedCapsuleV2, error) {
	var result VerifiedCapsuleV2

	info, err := os.Lstat(filename)
	if err != nil {
		return result, fmt.Errorf("capsule: stat archive: %w", err)
	}

	if !info.Mode().IsRegular() ||
		info.Size() > MaxCapsuleSize {
		return result, fmt.Errorf("capsule: invalid archive file or size")
	}

	f, err := os.Open(filename)
	if err != nil {
		return result, fmt.Errorf("capsule: open archive: %w", err)
	}
	defer f.Close()

	tr := tar.NewReader(io.LimitReader(f, MaxCapsuleSize+1))

	header, err := tr.Next()
	if err != nil {
		return result, fmt.Errorf("capsule: read manifest header: %w", err)
	}

	if header.Name != "manifest.json" ||
		header.Typeflag != tar.TypeReg ||
		header.Size < 0 ||
		header.Size > MaxManifestSize {
		return result, fmt.Errorf("capsule: invalid manifest entry")
	}

	data, err := io.ReadAll(io.LimitReader(tr, MaxManifestSize+1))
	if err != nil {
		return result, fmt.Errorf("capsule: read manifest: %w", err)
	}

	if int64(len(data)) != header.Size {
		return result, fmt.Errorf("capsule: manifest size mismatch")
	}

	if err := json.Unmarshal(data, &result.Manifest); err != nil {
		return result, fmt.Errorf("capsule: decode manifest: %w", err)
	}

	if result.Manifest.SchemaVersion != SchemaVersionV2 {
		return result, fmt.Errorf("capsule: expected schema version 2")
	}

	if err := result.Manifest.Validate(); err != nil {
		return result, fmt.Errorf("capsule: invalid manifest: %w", err)
	}

	expectedSize := expectedTarSize(
		int64(len(data)),
		result.Manifest.Workspace,
	)

	if info.Size() != expectedSize {
		return VerifiedCapsuleV2{}, fmt.Errorf(
			"capsule: archive size mismatch: expected %d bytes, got %d",
			expectedSize,
			info.Size(),
		)
	}

	metadata := result.Manifest.Workspace.Files
	result.Files = make([]WorkspaceFile, 0, len(metadata))

	for _, file := range metadata {
		header, err := tr.Next()
		if err != nil {
			return VerifiedCapsuleV2{}, fmt.Errorf(
				"capsule: read workspace entry %q: %w",
				file.Path, err,
			)
		}

		if header.Name != "workspace/"+file.Path ||
			header.Typeflag != tar.TypeReg ||
			header.Size != file.Size {
			return VerifiedCapsuleV2{}, fmt.Errorf(
				"capsule: invalid workspace entry: %q",
				file.Path,
			)
		}

		content, err := io.ReadAll(
			io.LimitReader(tr, MaxWorkspaceFileSize+1),
		)
		if err != nil {
			return VerifiedCapsuleV2{}, fmt.Errorf(
				"capsule: read workspace file %q: %w",
				file.Path, err,
			)
		}

		if int64(len(content)) != file.Size {
			return VerifiedCapsuleV2{}, fmt.Errorf(
				"capsule: workspace size mismatch: %q",
				file.Path,
			)
		}

		if err := VerifyWorkspaceFileV2(file, content); err != nil {
			return VerifiedCapsuleV2{}, fmt.Errorf(
				"capsule: verify %q: %w",
				file.Path, err,
			)
		}

		mode := os.FileMode(header.Mode)
		if err := ValidateWorkspacePath(file.Path, mode); err != nil {
			return VerifiedCapsuleV2{}, fmt.Errorf(
				"capsule: invalid workspace path or mode: %w",
				err,
			)
		}

		result.Files = append(result.Files, WorkspaceFile{
			Path: file.Path,
			Data: content,
			Mode: mode,
		})
	}

	if _, err := tr.Next(); err != io.EOF {
		return VerifiedCapsuleV2{}, fmt.Errorf(
			"capsule: unexpected extra entry or archive error: %v",
			err,
		)
	}

	return result, nil
}
