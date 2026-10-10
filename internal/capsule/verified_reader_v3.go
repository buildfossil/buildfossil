package capsule

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

type VerifiedCapsuleV3 struct {
	Manifest  Manifest
	Files     []WorkspaceFile
	Artifacts map[string][]byte
}

func ReadVerifiedV3(filename string) (VerifiedCapsuleV3, error) {
	var result VerifiedCapsuleV3

	info, err := os.Lstat(filename)
	if err != nil {
		return result, fmt.Errorf("capsule: stat v3 archive: %w", err)
	}

	if !info.Mode().IsRegular() ||
		info.Size() > MaxCapsuleSize {
		return result, fmt.Errorf("capsule: invalid v3 archive file or size")
	}

	f, err := os.Open(filename)
	if err != nil {
		return result, fmt.Errorf("capsule: open v3 archive: %w", err)
	}
	defer f.Close()

	tr := tar.NewReader(io.LimitReader(f, MaxCapsuleSize+1))

	header, err := tr.Next()
	if err != nil {
		return result, fmt.Errorf("capsule: read v3 manifest header: %w", err)
	}

	if header.Name != "manifest.json" ||
		header.Typeflag != tar.TypeReg ||
		header.Size < 0 ||
		header.Size > MaxManifestSize {
		return result, fmt.Errorf("capsule: invalid v3 manifest entry")
	}

	manifestData, err := io.ReadAll(
		io.LimitReader(tr, MaxManifestSize+1),
	)
	if err != nil {
		return result, fmt.Errorf("capsule: read v3 manifest: %w", err)
	}

	if int64(len(manifestData)) != header.Size {
		return result, fmt.Errorf("capsule: v3 manifest size mismatch")
	}

	if err := json.Unmarshal(manifestData, &result.Manifest); err != nil {
		return result, fmt.Errorf("capsule: decode v3 manifest: %w", err)
	}

	if result.Manifest.SchemaVersion != SchemaVersionV3 {
		return VerifiedCapsuleV3{}, fmt.Errorf(
			"capsule: expected schema version 3",
		)
	}

	if err := result.Manifest.Validate(); err != nil {
		return VerifiedCapsuleV3{}, fmt.Errorf(
			"capsule: invalid v3 manifest: %w",
			err,
		)
	}

	if len(result.Manifest.GoModules) != 1 {
		return VerifiedCapsuleV3{}, fmt.Errorf(
			"capsule: v3 reader requires exactly one Go module",
		)
	}

	expectedSize := expectedTarSize(
		int64(len(manifestData)),
		result.Manifest.Workspace,
	)

	for _, artifact := range result.Manifest.GoModules[0].Artifacts {
		expectedSize += tarEntrySize(artifact.Size)
	}

	if info.Size() != expectedSize {
		return VerifiedCapsuleV3{}, fmt.Errorf(
			"capsule: v3 archive size mismatch: expected %d, got %d",
			expectedSize,
			info.Size(),
		)
	}

	result.Files = make(
		[]WorkspaceFile,
		0,
		len(result.Manifest.Workspace.Files),
	)

	var goSum []byte
	foundGoSum := false

	for _, metadata := range result.Manifest.Workspace.Files {
		header, err := tr.Next()
		if err != nil {
			return VerifiedCapsuleV3{}, fmt.Errorf(
				"capsule: read v3 workspace entry %q: %w",
				metadata.Path,
				err,
			)
		}

		if header.Name != "workspace/"+metadata.Path ||
			header.Typeflag != tar.TypeReg ||
			header.Size != metadata.Size {
			return VerifiedCapsuleV3{}, fmt.Errorf(
				"capsule: invalid v3 workspace entry %q",
				metadata.Path,
			)
		}

		data, err := io.ReadAll(
			io.LimitReader(tr, MaxWorkspaceFileSize+1),
		)
		if err != nil {
			return VerifiedCapsuleV3{}, err
		}

		if err := VerifyWorkspaceFileV2(metadata, data); err != nil {
			return VerifiedCapsuleV3{}, err
		}

		mode := os.FileMode(header.Mode)
		if err := ValidateWorkspacePath(metadata.Path, mode); err != nil {
			return VerifiedCapsuleV3{}, err
		}

		result.Files = append(result.Files, WorkspaceFile{
			Path: metadata.Path,
			Data: data,
			Mode: mode,
		})

		if metadata.Path == "go.sum" {
			goSum = data
			foundGoSum = true
		}
	}

	if !foundGoSum {
		return VerifiedCapsuleV3{}, fmt.Errorf(
			"capsule: v3 archive missing workspace/go.sum",
		)
	}

	result.Artifacts = make(map[string][]byte, MaxGoModuleArtifactsV3)

	for _, name := range goArtifactNamesV3 {
		var metadata *GoModuleArtifact

		for i := range result.Manifest.GoModules[0].Artifacts {
			artifact := &result.Manifest.GoModules[0].Artifacts[i]
			if artifact.Path == name {
				metadata = artifact
				break
			}
		}

		if metadata == nil {
			return VerifiedCapsuleV3{}, fmt.Errorf(
				"capsule: missing artifact metadata %q",
				name,
			)
		}

		header, err := tr.Next()
		if err != nil {
			return VerifiedCapsuleV3{}, fmt.Errorf(
				"capsule: read artifact %q: %w",
				name,
				err,
			)
		}

		if header.Name != "dependencies/"+name ||
			header.Typeflag != tar.TypeReg ||
			header.Size != metadata.Size {
			return VerifiedCapsuleV3{}, fmt.Errorf(
				"capsule: invalid v3 artifact entry %q",
				name,
			)
		}

		data, err := io.ReadAll(
			io.LimitReader(tr, MaxGoModuleTotalSizeV3+1),
		)
		if err != nil {
			return VerifiedCapsuleV3{}, err
		}

		if int64(len(data)) != metadata.Size ||
			SHA256(data) != metadata.SHA256 {
			return VerifiedCapsuleV3{}, fmt.Errorf(
				"capsule: v3 artifact integrity mismatch: %q",
				name,
			)
		}

		result.Artifacts[name] = data
	}

	if _, err := tr.Next(); err != io.EOF {
		return VerifiedCapsuleV3{}, fmt.Errorf(
			"capsule: unexpected extra v3 entry or archive error: %v",
			err,
		)
	}

	if err := VerifyGoModuleArtifactsV3(
		result.Manifest.GoModules[0],
		result.Artifacts,
		goSum,
	); err != nil {
		return VerifiedCapsuleV3{}, fmt.Errorf(
			"capsule: verify v3 Go module: %w",
			err,
		)
	}

	return result, nil
}
