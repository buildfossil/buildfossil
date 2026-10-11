package capsule

import (
	"bytes"
	"fmt"
	"io"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
	"golang.org/x/mod/sumdb/dirhash"
)

// VerifyGoGraphModFileV3 verifies an additional module go.mod
// needed to reconstruct the dependency graph offline.
func VerifyGoGraphModFileV3(
	file GoGraphModFileV3,
	data []byte,
	goSum []byte,
) error {
	if err := module.Check(file.Path, file.Version); err != nil {
		return fmt.Errorf("capsule: invalid graph module identity: %w", err)
	}

	if file.Artifact.Size < 0 ||
		file.Artifact.Size > MaxGoGraphModTotalSizeV3 {
		return fmt.Errorf("capsule: graph mod size limit exceeded")
	}

	if int64(len(data)) != file.Artifact.Size {
		return fmt.Errorf("capsule: graph mod size mismatch")
	}

	if SHA256(data) != file.Artifact.SHA256 {
		return fmt.Errorf("capsule: graph mod SHA-256 mismatch")
	}

	parsed, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		return fmt.Errorf("capsule: invalid graph go.mod: %w", err)
	}

	if parsed.Module == nil ||
		parsed.Module.Mod.Path != file.Path {
		return fmt.Errorf("capsule: graph module path mismatch")
	}

	modHash, err := dirhash.Hash1(
		[]string{"go.mod"},
		func(name string) (io.ReadCloser, error) {
			if name != "go.mod" {
				return nil, fmt.Errorf("unexpected graph mod file: %s", name)
			}

			return io.NopCloser(bytes.NewReader(data)), nil
		},
	)
	if err != nil {
		return fmt.Errorf("capsule: hash graph go.mod: %w", err)
	}

	if err := verifyGoSumEntry(
		goSum,
		file.Path+" "+file.Version+"/go.mod",
		modHash,
	); err != nil {
		return fmt.Errorf("capsule: verify graph go.mod checksum: %w", err)
	}

	return nil
}
