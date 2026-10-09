package capsule

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

func WriteWorkspaceV2(
	w io.Writer,
	manifest Manifest,
	files []WorkspaceFile,
) error {
	if manifest.SchemaVersion != SchemaVersionV2 {
		return fmt.Errorf("capsule: v2 writer requires schema version 2")
	}

	metadata := make([]FileMetadata, 0, len(files))
	for _, file := range files {
		if err := ValidateWorkspacePath(file.Path, file.Mode); err != nil {
			return fmt.Errorf("capsule: invalid workspace file: %w", err)
		}

		if int64(len(file.Data)) > MaxWorkspaceFileSize {
			return fmt.Errorf("capsule: workspace file too large: %q", file.Path)
		}

		metadata = append(metadata, FileMetadata{
			Path:   file.Path,
			Size:   int64(len(file.Data)),
			SHA256: SHA256(file.Data),
		})
	}

	manifest.Workspace = Workspace{Files: metadata}

	if err := manifest.Validate(); err != nil {
		return fmt.Errorf("capsule: invalid v2 manifest: %w", err)
	}

	data, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("capsule: encode manifest: %w", err)
	}

	if len(data) > MaxManifestSize {
		return fmt.Errorf("capsule: manifest too large")
	}

	var archive bytes.Buffer
	tw := tar.NewWriter(&archive)

	writeEntry := func(name string, content []byte, mode int64) error {
		header := &tar.Header{
			Name:     name,
			Mode:     mode,
			Size:     int64(len(content)),
			Typeflag: tar.TypeReg,
		}

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		_, err := tw.Write(content)
		return err
	}

	if err := writeEntry("manifest.json", data, 0600); err != nil {
		return fmt.Errorf("capsule: write manifest: %w", err)
	}

	for _, file := range files {
		if err := writeEntry(
			"workspace/"+file.Path,
			file.Data,
			int64(file.Mode.Perm()),
		); err != nil {
			return fmt.Errorf("capsule: write %q: %w", file.Path, err)
		}
	}

	if err := tw.Close(); err != nil {
		return fmt.Errorf("capsule: finalize archive: %w", err)
	}

	if int64(archive.Len()) > MaxCapsuleSize {
		return fmt.Errorf("capsule: archive exceeds size limit")
	}

	if _, err := io.Copy(w, &archive); err != nil {
		return fmt.Errorf("capsule: write output: %w", err)
	}

	return nil
}
