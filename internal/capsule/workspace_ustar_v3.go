package capsule

import (
	"archive/tar"
	"fmt"
	"io"
)

// validateWorkspaceUSTARPathV3 requires a workspace entry name that
// can be represented as a single USTAR header without PAX records.
func validateWorkspaceUSTARPathV3(name string) error {
	header := &tar.Header{
		Name:     "workspace/" + name,
		Mode:     0600,
		Size:     0,
		Typeflag: tar.TypeReg,
		Format:   tar.FormatUSTAR,
	}

	writer := tar.NewWriter(io.Discard)
	if err := writer.WriteHeader(header); err != nil {
		return fmt.Errorf("capsule: workspace path is not USTAR-compatible: %w", err)
	}
	return writer.Close()
}
