package capsule

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

var goArtifactNamesV3 = []string{
	"module-0001.zip",
	"module-0001.mod",
	"module-0001.info",
}

// WriteWorkspaceV3 writes a schema-v3 capsule containing a workspace
// and exactly one verified Go module.
//
// It validates all inputs before writing any bytes to w.
func WriteWorkspaceV3(
	w io.Writer,
	manifest Manifest,
	files []WorkspaceFile,
	artifacts map[string][]byte,
) error {
	if manifest.SchemaVersion != SchemaVersionV3 {
		return fmt.Errorf("capsule: v3 writer requires schema version 3")
	}

	if len(manifest.GoModules) != 1 {
		return fmt.Errorf("capsule: v3 writer requires exactly one Go module")
	}

	metadata := make([]FileMetadata, 0, len(files))

	var goSum []byte
	foundGoSum := false

	for _, file := range files {
		if err := ValidateWorkspacePath(file.Path, file.Mode); err != nil {
			return fmt.Errorf(
				"capsule: invalid workspace file: %w",
				err,
			)
		}

		if int64(len(file.Data)) > MaxWorkspaceFileSize {
			return fmt.Errorf(
				"capsule: workspace file too large: %q",
				file.Path,
			)
		}

		metadata = append(metadata, FileMetadata{
			Path:   file.Path,
			Size:   int64(len(file.Data)),
			SHA256: SHA256(file.Data),
		})

		if file.Path == "go.sum" {
			if foundGoSum {
				return fmt.Errorf("capsule: duplicate workspace go.sum")
			}

			goSum = file.Data
			foundGoSum = true
		}
	}

	if !foundGoSum {
		return fmt.Errorf("capsule: v3 requires workspace/go.sum")
	}

	manifest.Workspace = Workspace{Files: metadata}

	if err := manifest.Validate(); err != nil {
		return fmt.Errorf("capsule: invalid v3 manifest: %w", err)
	}

	if err := VerifyGoModuleArtifactsV3(
		manifest.GoModules[0],
		artifacts,
		goSum,
	); err != nil {
		return fmt.Errorf("capsule: verify Go module: %w", err)
	}

	manifestData, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("capsule: encode manifest: %w", err)
	}

	if len(manifestData) > MaxManifestSize {
		return fmt.Errorf("capsule: manifest too large")
	}

	var archive bytes.Buffer
	tw := tar.NewWriter(&archive)

	writeEntry := func(name string, data []byte, mode int64) error {
		header := &tar.Header{
			Name:     name,
			Mode:     mode,
			Size:     int64(len(data)),
			Typeflag: tar.TypeReg,
		}

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		_, err := tw.Write(data)
		return err
	}

	if err := writeEntry("manifest.json", manifestData, 0600); err != nil {
		return fmt.Errorf("capsule: write manifest: %w", err)
	}

	for _, file := range files {
		if err := writeEntry(
			"workspace/"+file.Path,
			file.Data,
			int64(file.Mode.Perm()),
		); err != nil {
			return fmt.Errorf(
				"capsule: write workspace file %q: %w",
				file.Path,
				err,
			)
		}
	}

	for _, name := range goArtifactNamesV3 {
		data, ok := artifacts[name]
		if !ok {
			return fmt.Errorf("capsule: missing Go artifact %q", name)
		}

		if err := writeEntry(
			"dependencies/"+name,
			data,
			0600,
		); err != nil {
			return fmt.Errorf(
				"capsule: write Go artifact %q: %w",
				name,
				err,
			)
		}
	}

	if err := tw.Close(); err != nil {
		return fmt.Errorf("capsule: finalize v3 archive: %w", err)
	}

	if int64(archive.Len()) > MaxCapsuleSize {
		return fmt.Errorf("capsule: v3 archive exceeds size limit")
	}

	if _, err := io.Copy(w, &archive); err != nil {
		return fmt.Errorf("capsule: write v3 archive: %w", err)
	}

	return nil
}
