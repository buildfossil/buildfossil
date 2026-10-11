package capsule

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"syscall"
)

type VerifiedCapsuleV3 struct {
	Manifest  Manifest
	Files     []WorkspaceFile
	Artifacts map[string][]byte
}

func ReadVerifiedV3(filename string) (VerifiedCapsuleV3, error) {
	var result VerifiedCapsuleV3

	f, err := os.OpenFile(filename, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return result, fmt.Errorf("capsule: open v3 archive: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return result, fmt.Errorf("capsule: stat opened v3 archive: %w", err)
	}

	if !info.Mode().IsRegular() ||
		info.Size() > MaxCapsuleSize {
		return result, fmt.Errorf("capsule: invalid v3 archive file or size")
	}

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

	if len(result.Manifest.GoModules) > MaxGoModulesV3 {
		return VerifiedCapsuleV3{}, fmt.Errorf(
			"capsule: too many Go modules",
		)
	}

	expectedSize := expectedTarSize(
		int64(len(manifestData)),
		result.Manifest.Workspace,
	)

	for _, mod := range result.Manifest.GoModules {
		for _, artifact := range mod.Artifacts {
			expectedSize += tarEntrySize(artifact.Size)
		}
	}

	for _, graphFile := range result.Manifest.GoGraphModFiles {
		expectedSize += tarEntrySize(graphFile.Artifact.Size)
	}

	for _, infoFile := range result.Manifest.GoGraphInfoFiles {
		expectedSize += tarEntrySize(infoFile.Artifact.Size)
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

	var goMod []byte
	var goSum []byte
	foundGoMod := false
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

		switch metadata.Path {
		case "go.mod":
			goMod = data
			foundGoMod = true

		case "go.sum":
			goSum = data
			foundGoSum = true
		}
	}

	if !foundGoSum {
		return VerifiedCapsuleV3{}, fmt.Errorf(
			"capsule: v3 archive missing workspace/go.sum",
		)
	}

	if !foundGoMod {
		return VerifiedCapsuleV3{}, fmt.Errorf(
			"capsule: v3 archive missing workspace/go.mod",
		)
	}

	if err := ValidateGoSelectedModuleConsistencyV3(
		goMod,
		result.Manifest,
	); err != nil {
		return VerifiedCapsuleV3{}, fmt.Errorf(
			"capsule: invalid v3 Go dependency declaration: %w",
			err,
		)
	}

	result.Artifacts = make(map[string][]byte)

	var totalArtifactSize int64

	for _, mod := range result.Manifest.GoModules {
		for _, metadata := range mod.Artifacts {
			name := metadata.Path

			header, err := tr.Next()
			if err != nil {
				return VerifiedCapsuleV3{}, fmt.Errorf(
					"capsule: read artifact %q: %w",
					name, err,
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

			remaining := MaxGoModuleTotalSizeV3 - totalArtifactSize
			if remaining < 0 || metadata.Size > remaining {
				return VerifiedCapsuleV3{}, fmt.Errorf(
					"capsule: Go module artifacts exceed total size limit",
				)
			}

			data, err := io.ReadAll(
				io.LimitReader(tr, remaining+1),
			)
			if err != nil {
				return VerifiedCapsuleV3{}, fmt.Errorf(
					"capsule: read artifact data %q: %w",
					name, err,
				)
			}

			if int64(len(data)) != metadata.Size ||
				SHA256(data) != metadata.SHA256 {
				return VerifiedCapsuleV3{}, fmt.Errorf(
					"capsule: v3 artifact integrity mismatch: %q",
					name,
				)
			}

			if _, exists := result.Artifacts[name]; exists {
				return VerifiedCapsuleV3{}, fmt.Errorf(
					"capsule: duplicate Go artifact %q",
					name,
				)
			}

			result.Artifacts[name] = data
			totalArtifactSize += int64(len(data))
		}
	}

	if err := VerifyGoModuleArtifactSetV3(
		result.Manifest.GoModules,
		result.Artifacts,
		goSum,
	); err != nil {
		return VerifiedCapsuleV3{}, fmt.Errorf(
			"capsule: verify v3 Go module set: %w",
			err,
		)
	}

	// Read additional graph-only go.mod files after selected modules.
	graphArtifacts := make(map[string][]byte, len(result.Manifest.GoGraphModFiles))

	for _, graphFile := range result.Manifest.GoGraphModFiles {
		metadata := graphFile.Artifact
		name := metadata.Path

		header, err := tr.Next()
		if err != nil {
			return VerifiedCapsuleV3{}, fmt.Errorf(
				"capsule: read graph artifact %q: %w", name, err,
			)
		}

		if header.Name != "dependencies/"+name ||
			header.Typeflag != tar.TypeReg ||
			header.Size != metadata.Size {
			return VerifiedCapsuleV3{}, fmt.Errorf(
				"capsule: invalid graph artifact entry %q", name,
			)
		}

		data, err := io.ReadAll(
			io.LimitReader(tr, MaxGoGraphModTotalSizeV3+1),
		)
		if err != nil {
			return VerifiedCapsuleV3{}, fmt.Errorf(
				"capsule: read graph artifact %q: %w", name, err,
			)
		}

		if int64(len(data)) != metadata.Size {
			return VerifiedCapsuleV3{}, fmt.Errorf(
				"capsule: graph artifact size mismatch %q", name,
			)
		}

		if err := VerifyGoGraphModFileV3(
			graphFile, data, goSum,
		); err != nil {
			return VerifiedCapsuleV3{}, fmt.Errorf(
				"capsule: invalid graph artifact %q: %w", name, err,
			)
		}

		if _, exists := result.Artifacts[name]; exists {
			return VerifiedCapsuleV3{}, fmt.Errorf(
				"capsule: duplicate graph artifact %q", name,
			)
		}

		graphArtifacts[name] = data
		result.Artifacts[name] = data
	}

	if err := VerifyGoGraphModArtifactSetV3(
		result.Manifest.GoGraphModFiles,
		graphArtifacts,
		goSum,
	); err != nil {
		return VerifiedCapsuleV3{}, fmt.Errorf(
			"capsule: verify graph artifacts: %w", err,
		)
	}

	// Read metadata-only .info files after graph .mod files.
	infoArtifacts := make(
		map[string][]byte,
		len(result.Manifest.GoGraphInfoFiles),
	)

	var totalInfoSize int64

	for _, infoFile := range result.Manifest.GoGraphInfoFiles {
		metadata := infoFile.Artifact
		name := metadata.Path

		header, err := tr.Next()
		if err != nil {
			return VerifiedCapsuleV3{}, fmt.Errorf(
				"capsule: read graph info %q: %w", name, err,
			)
		}

		if header.Name != "dependencies/"+name ||
			header.Typeflag != tar.TypeReg ||
			header.Size != metadata.Size {
			return VerifiedCapsuleV3{}, fmt.Errorf(
				"capsule: invalid graph info TAR entry %q", name,
			)
		}

		remaining := MaxGoGraphInfoTotalSizeV3 - totalInfoSize
		if remaining < 0 || metadata.Size > remaining {
			return VerifiedCapsuleV3{}, fmt.Errorf(
				"capsule: graph info total size exceeded",
			)
		}

		data, err := io.ReadAll(
			io.LimitReader(tr, remaining+1),
		)
		if err != nil {
			return VerifiedCapsuleV3{}, fmt.Errorf(
				"capsule: read graph info data %q: %w", name, err,
			)
		}

		if int64(len(data)) != metadata.Size {
			return VerifiedCapsuleV3{}, fmt.Errorf(
				"capsule: graph info size mismatch %q", name,
			)
		}

		if err := VerifyGoGraphInfoFileV3(
			infoFile, data,
		); err != nil {
			return VerifiedCapsuleV3{}, fmt.Errorf(
				"capsule: invalid graph info %q: %w", name, err,
			)
		}

		if _, exists := result.Artifacts[name]; exists {
			return VerifiedCapsuleV3{}, fmt.Errorf(
				"capsule: duplicate graph info artifact %q", name,
			)
		}

		infoArtifacts[name] = data
		result.Artifacts[name] = data
		totalInfoSize += int64(len(data))
	}

	if err := VerifyGoGraphInfoArtifactSetV3(
		result.Manifest.GoGraphInfoFiles,
		infoArtifacts,
	); err != nil {
		return VerifiedCapsuleV3{}, fmt.Errorf(
			"capsule: verify graph info artifacts: %w", err,
		)
	}

	// Reject unexpected dependency entries, including when there
	// are zero declared modules.
	if _, err := tr.Next(); err != io.EOF {
		return VerifiedCapsuleV3{}, fmt.Errorf(
			"capsule: unexpected extra v3 entry or archive error: %v",
			err,
		)
	}

	return result, nil
}
