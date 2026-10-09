package capsule

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

type VerifiedCapsule struct {
	Manifest Manifest
	File     WorkspaceFile
}

func ReadVerified(filename string) (VerifiedCapsule, error) {
	var result VerifiedCapsule

	info, err := os.Lstat(filename)
	if err != nil {
		return result, fmt.Errorf("capsule: stat archive: %w", err)
	}

	if !info.Mode().IsRegular() || info.Size() > MaxCapsuleSize {
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
	if err != nil || int64(len(data)) != header.Size {
		return result, fmt.Errorf("capsule: invalid manifest content: %v", err)
	}

	if err := json.Unmarshal(data, &result.Manifest); err != nil {
		return result, fmt.Errorf("capsule: decode manifest: %w", err)
	}

	if err := result.Manifest.Validate(); err != nil {
		return result, err
	}

	if len(result.Manifest.Workspace.Files) != 1 {
		return result, fmt.Errorf("capsule: expected exactly one workspace file")
	}

	metadata := result.Manifest.Workspace.Files[0]

	header, err = tr.Next()
	if err != nil {
		return result, fmt.Errorf("capsule: read workspace header: %w", err)
	}

	if header.Name != "workspace/"+metadata.Path ||
		header.Typeflag != tar.TypeReg ||
		header.Size != metadata.Size {
		return result, fmt.Errorf("capsule: invalid workspace entry")
	}

	content, err := io.ReadAll(io.LimitReader(tr, MaxWorkspaceFileSize+1))
	if err != nil {
		return result, fmt.Errorf("capsule: read workspace: %w", err)
	}

	if err := VerifyWorkspaceFile(metadata, content); err != nil {
		return result, err
	}

	result.File = WorkspaceFile{
		Path: metadata.Path,
		Data: content,
		Mode: os.FileMode(header.Mode),
	}

	if err := ValidateWorkspacePath(result.File.Path, result.File.Mode); err != nil {
		return VerifiedCapsule{}, err
	}

	if _, err := tr.Next(); err != io.EOF {
		return VerifiedCapsule{}, fmt.Errorf("capsule: unexpected extra entry or archive error: %v", err)
	}

	return result, nil
}
