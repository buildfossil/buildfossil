package capsule

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

const MaxCapsuleSize int64 = 16 << 20

func ReadManifest(filename string) (Manifest, error) {
	var manifest Manifest

	info, err := os.Lstat(filename)
	if err != nil {
		return manifest, fmt.Errorf("capsule: stat: %w", err)
	}

	if !info.Mode().IsRegular() {
		return manifest, fmt.Errorf("capsule: not a regular file")
	}

	if info.Size() > MaxCapsuleSize {
		return manifest, fmt.Errorf("capsule: archive too large")
	}

	f, err := os.Open(filename)
	if err != nil {
		return manifest, fmt.Errorf("capsule: open: %w", err)
	}
	defer f.Close()

	tr := tar.NewReader(f)

	header, err := tr.Next()
	if err != nil {
		return manifest, fmt.Errorf("capsule: read header: %w", err)
	}

	if header.Name != "manifest.json" ||
		header.Typeflag != tar.TypeReg {
		return manifest, fmt.Errorf("capsule: invalid manifest entry")
	}

	if header.Size < 0 || header.Size > MaxManifestSize {
		return manifest, fmt.Errorf("capsule: invalid manifest size")
	}

	data, err := io.ReadAll(io.LimitReader(tr, MaxManifestSize+1))
	if err != nil {
		return manifest, fmt.Errorf("capsule: read manifest: %w", err)
	}

	if int64(len(data)) != header.Size {
		return manifest, fmt.Errorf("capsule: manifest size mismatch")
	}

	if err := json.Unmarshal(data, &manifest); err != nil {
		return manifest, fmt.Errorf("capsule: decode manifest: %w", err)
	}

	if err := manifest.Validate(); err != nil {
		return manifest, err
	}

	return manifest, nil
}
