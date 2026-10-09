package capsule

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"io"
)

const MaxManifestSize = 1 << 20 // 1 MiB

func Write(w io.Writer, manifest Manifest) error {
	if err := manifest.Validate(); err != nil {
		return fmt.Errorf("capsule: invalid manifest: %w", err)
	}

	data, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("capsule: encode manifest: %w", err)
	}

	if len(data) > MaxManifestSize {
		return fmt.Errorf("capsule: manifest exceeds size limit")
	}

	tw := tar.NewWriter(w)

	header := &tar.Header{
		Name:     "manifest.json",
		Mode:     0600,
		Size:     int64(len(data)),
		Typeflag: tar.TypeReg,
	}

	if err := tw.WriteHeader(header); err != nil {
		return fmt.Errorf("capsule: write header: %w", err)
	}

	if _, err := tw.Write(data); err != nil {
		return fmt.Errorf("capsule: write manifest: %w", err)
	}

	if err := tw.Close(); err != nil {
		return fmt.Errorf("capsule: finalize archive: %w", err)
	}

	return nil
}
