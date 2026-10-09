package capsule

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"io"
)

func WriteWithWorkspace(
	w io.Writer,
	manifest Manifest,
	file WorkspaceFile,
) error {
	if err := manifest.Validate(); err != nil {
		return fmt.Errorf("capsule: invalid manifest: %w", err)
	}

	// Only the controlled fixture is supported in this experiment.
	if file.Path != "fixture.txt" {
		return fmt.Errorf("capsule: unsupported workspace file")
	}

	if err := ValidateWorkspacePath(file.Path, file.Mode); err != nil {
		return err
	}

	if len(file.Data) > int(MaxWorkspaceFileSize) {
		return fmt.Errorf("capsule: workspace file exceeds size limit")
	}

	manifest.Workspace = Workspace{
		Files: []FileMetadata{
			{
				Path:   file.Path,
				Size:   int64(len(file.Data)),
				SHA256: SHA256(file.Data),
			},
		},
	}

	if err := manifest.Validate(); err != nil {
		return fmt.Errorf("capsule: invalid generated manifest: %w", err)
	}

	data, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("capsule: encode manifest: %w", err)
	}

	if len(data) > MaxManifestSize {
		return fmt.Errorf("capsule: manifest exceeds size limit")
	}

	tw := tar.NewWriter(w)

	entries := []struct {
		name string
		data []byte
		mode int64
	}{
		{"manifest.json", data, 0600},
		{"workspace/" + file.Path, file.Data, int64(file.Mode.Perm())},
	}

	for _, entry := range entries {
		header := &tar.Header{
			Name:     entry.name,
			Mode:     entry.mode,
			Size:     int64(len(entry.data)),
			Typeflag: tar.TypeReg,
		}

		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("capsule: write %s header: %w", entry.name, err)
		}

		if _, err := tw.Write(entry.data); err != nil {
			return fmt.Errorf("capsule: write %s: %w", entry.name, err)
		}
	}

	if err := tw.Close(); err != nil {
		return fmt.Errorf("capsule: finalize archive: %w", err)
	}

	return nil
}
