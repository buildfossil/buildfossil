package capsule

import (
	"archive/tar"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
)

// VerifiedStoredCapsuleV3 owns a disk-backed artifact store.
// The caller must call Close when finished.
type VerifiedStoredCapsuleV3 struct {
	Manifest Manifest
	Files    []WorkspaceFile
	Store    *ArtifactStoreV3
}

func (v *VerifiedStoredCapsuleV3) Close() error {
	if v == nil || v.Store == nil {
		return nil
	}
	err := v.Store.Close()
	v.Store = nil
	return err
}

// ReadVerifiedToStoreV3 reads a schema-v3 capsule and places
// dependencies on disk instead of retaining their ZIP bytes in RAM.
//
// Workspace files remain bounded in memory.
// All dependencies are verified before this function succeeds.
func ReadVerifiedToStoreV3(
	filename string,
) (result VerifiedStoredCapsuleV3, resultErr error) {

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
		info.Size() > MaxCapsuleSizeV3 {
		return result, fmt.Errorf("capsule: invalid v3 archive file or size")
	}

	tr := tar.NewReader(io.LimitReader(f, MaxCapsuleSizeV3+1))

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
		return VerifiedStoredCapsuleV3{}, fmt.Errorf(
			"capsule: expected schema version 3",
		)
	}

	if err := result.Manifest.Validate(); err != nil {
		return VerifiedStoredCapsuleV3{}, fmt.Errorf(
			"capsule: invalid v3 manifest: %w",
			err,
		)
	}

	if len(result.Manifest.GoModules) > MaxGoModulesV3 {
		return VerifiedStoredCapsuleV3{}, fmt.Errorf(
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
		return VerifiedStoredCapsuleV3{}, fmt.Errorf(
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
			return VerifiedStoredCapsuleV3{}, fmt.Errorf(
				"capsule: read v3 workspace entry %q: %w",
				metadata.Path,
				err,
			)
		}

		if header.Name != "workspace/"+metadata.Path ||
			header.Typeflag != tar.TypeReg ||
			header.Size != metadata.Size {
			return VerifiedStoredCapsuleV3{}, fmt.Errorf(
				"capsule: invalid v3 workspace entry %q",
				metadata.Path,
			)
		}

		data, err := io.ReadAll(
			io.LimitReader(tr, MaxWorkspaceFileSize+1),
		)
		if err != nil {
			return VerifiedStoredCapsuleV3{}, err
		}

		if err := VerifyWorkspaceFileV2(metadata, data); err != nil {
			return VerifiedStoredCapsuleV3{}, err
		}

		mode := os.FileMode(header.Mode)
		if err := ValidateWorkspacePath(metadata.Path, mode); err != nil {
			return VerifiedStoredCapsuleV3{}, err
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
		return VerifiedStoredCapsuleV3{}, fmt.Errorf(
			"capsule: v3 archive missing workspace/go.sum",
		)
	}

	if !foundGoMod {
		return VerifiedStoredCapsuleV3{}, fmt.Errorf(
			"capsule: v3 archive missing workspace/go.mod",
		)
	}

	if err := ValidateGoSelectedModuleConsistencyV3(
		goMod,
		result.Manifest,
	); err != nil {
		return VerifiedStoredCapsuleV3{}, fmt.Errorf(
			"capsule: invalid v3 Go dependency declaration: %w",
			err,
		)
	}

	budget := MaxGoModuleTotalSizeV3 +
		MaxGoGraphModTotalSizeV3 +
		MaxGoGraphInfoTotalSizeV3

	store, err := NewArtifactStoreV3(budget)
	if err != nil {
		return VerifiedStoredCapsuleV3{}, err
	}
	result.Store = store

	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, store.Close())
			result = VerifiedStoredCapsuleV3{}
		}
	}()

	// Read exactly one declared TAR entry into the artifact store.
	readArtifact := func(metadata GoModuleArtifact) error {
		header, err := tr.Next()
		if err != nil {
			return fmt.Errorf(
				"capsule: read artifact %q: %w",
				metadata.Path, err,
			)
		}

		if header.Name != "dependencies/"+metadata.Path ||
			header.Typeflag != tar.TypeReg ||
			header.Size != metadata.Size {
			return fmt.Errorf(
				"capsule: invalid artifact TAR entry %q",
				metadata.Path,
			)
		}

		if !strings.HasPrefix(metadata.SHA256, "sha256:") {
			return fmt.Errorf(
				"capsule: invalid artifact digest %q", metadata.Path,
			)
		}

		if err := store.Put(
			metadata.Path,
			tr,
			metadata.Size,
			strings.TrimPrefix(metadata.SHA256, "sha256:"),
		); err != nil {
			return fmt.Errorf(
				"capsule: store artifact %q: %w",
				metadata.Path, err,
			)
		}

		return nil
	}

	var selectedTotal int64
	for _, mod := range result.Manifest.GoModules {
		for _, metadata := range mod.Artifacts {
			if metadata.Size < 0 ||
				metadata.Size > MaxGoModuleTotalSizeV3-selectedTotal {
				return VerifiedStoredCapsuleV3{},
					fmt.Errorf("capsule: Go module artifact budget exceeded")
			}
			if err := readArtifact(metadata); err != nil {
				return VerifiedStoredCapsuleV3{}, err
			}
			selectedTotal += metadata.Size
		}
	}

	var graphModTotal int64
	for _, graph := range result.Manifest.GoGraphModFiles {
		metadata := graph.Artifact
		if metadata.Size < 0 ||
			metadata.Size > MaxGoGraphModTotalSizeV3-graphModTotal {
			return VerifiedStoredCapsuleV3{},
				fmt.Errorf("capsule: graph mod artifact budget exceeded")
		}
		if err := readArtifact(metadata); err != nil {
			return VerifiedStoredCapsuleV3{}, err
		}
		graphModTotal += metadata.Size
	}

	var graphInfoTotal int64
	for _, graph := range result.Manifest.GoGraphInfoFiles {
		metadata := graph.Artifact
		if metadata.Size < 0 ||
			metadata.Size > MaxGoGraphInfoTotalSizeV3-graphInfoTotal {
			return VerifiedStoredCapsuleV3{},
				fmt.Errorf("capsule: graph info artifact budget exceeded")
		}
		if err := readArtifact(metadata); err != nil {
			return VerifiedStoredCapsuleV3{}, err
		}
		graphInfoTotal += metadata.Size
	}

	if _, err := tr.Next(); err != io.EOF {
		return VerifiedStoredCapsuleV3{}, fmt.Errorf(
			"capsule: unexpected extra v3 entry or archive error: %v", err,
		)
	}

	if err := verifyCompleteGoArtifactStoreV3(
		result.Manifest, goSum, store,
	); err != nil {
		return VerifiedStoredCapsuleV3{}, fmt.Errorf(
			"capsule: verify disk-backed artifacts: %w", err,
		)
	}

	return result, nil
}
