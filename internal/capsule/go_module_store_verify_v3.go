package capsule

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
	"golang.org/x/mod/sumdb/dirhash"
)

// verifyGoModuleFromStoreV3 verifies a full selected Go module
// without loading its ZIP into memory.
//
// The store must remain open and must not be concurrently modified.
// As with other temporary-file verification, this does not defend
// against an attacker with arbitrary access under the same OS UID.
func verifyGoModuleFromStoreV3(
	mod GoModuleV3,
	index int,
	goSum []byte,
	store *ArtifactStoreV3,
) error {
	if store == nil || store.closed {
		return fmt.Errorf("capsule: module artifact store unavailable")
	}

	if index < 1 || index > MaxGoModulesV3 {
		return fmt.Errorf("capsule: invalid Go module index")
	}

	if err := module.Check(mod.Path, mod.Version); err != nil {
		return fmt.Errorf("capsule: invalid Go module identity: %w", err)
	}

	if len(mod.Artifacts) != MaxGoModuleArtifactsV3 {
		return fmt.Errorf("capsule: invalid Go artifact count")
	}

	prefix := fmt.Sprintf("module-%04d", index)
	names := map[string]bool{
		prefix + ".zip":  false,
		prefix + ".mod":  false,
		prefix + ".info": false,
	}

	var total int64
	var modData, infoData []byte
	var zipName string

	for _, metadata := range mod.Artifacts {
		seen, ok := names[metadata.Path]
		if !ok || seen {
			return fmt.Errorf(
				"capsule: invalid Go artifact path %q", metadata.Path,
			)
		}
		names[metadata.Path] = true

		if metadata.Size < 0 ||
			metadata.Size > MaxGoModuleTotalSizeV3-total {
			return fmt.Errorf("capsule: Go artifact size limit exceeded")
		}
		total += metadata.Size

		if !strings.HasPrefix(metadata.SHA256, "sha256:") ||
			len(metadata.SHA256) != len("sha256:")+64 {
			return fmt.Errorf("capsule: invalid Go artifact SHA-256")
		}

		record, ok := store.names[metadata.Path]
		if !ok ||
			record.size != metadata.Size ||
			record.digest != strings.TrimPrefix(metadata.SHA256, "sha256:") {
			return fmt.Errorf(
				"capsule: Go artifact metadata mismatch %q", metadata.Path,
			)
		}

		file, err := store.Open(metadata.Path)
		if err != nil {
			return fmt.Errorf(
				"capsule: open verified Go artifact %q: %w",
				metadata.Path, err,
			)
		}

		switch metadata.Path {
		case prefix + ".zip":
			zipName = metadata.Path
			err = file.Close()

		default:
			var data []byte
			data, err = io.ReadAll(io.LimitReader(
				file, metadata.Size+1,
			))
			closeErr := file.Close()
			if err == nil {
				err = closeErr
			}
			if err == nil && int64(len(data)) != metadata.Size {
				err = fmt.Errorf("Go artifact size mismatch")
			}
			if err == nil {
				switch metadata.Path {
				case prefix + ".mod":
					modData = data
				case prefix + ".info":
					infoData = data
				}
			}
		}

		if err != nil {
			return fmt.Errorf(
				"capsule: read verified artifact %q: %w",
				metadata.Path, err,
			)
		}
	}

	for name, seen := range names {
		if !seen {
			return fmt.Errorf("capsule: missing Go artifact %q", name)
		}
	}

	parsed, err := modfile.Parse("go.mod", modData, nil)
	if err != nil {
		return fmt.Errorf("capsule: invalid module go.mod: %w", err)
	}
	if parsed.Module == nil || parsed.Module.Mod.Path != mod.Path {
		return fmt.Errorf("capsule: module path mismatch")
	}

	var info struct {
		Version string `json:"Version"`
	}
	if err := json.Unmarshal(infoData, &info); err != nil {
		return fmt.Errorf("capsule: invalid module info: %w", err)
	}
	if info.Version != mod.Version {
		return fmt.Errorf("capsule: module version mismatch")
	}

	zipFile, err := store.Open(zipName)
	if err != nil {
		return fmt.Errorf("capsule: open module ZIP: %w", err)
	}
	defer zipFile.Close()

	zipHash, err := verifyGoModuleZipFileV3(
		zipFile.Name(), mod, modData,
	)
	if err != nil {
		return err
	}

	modHash, err := dirhash.Hash1(
		[]string{"go.mod"},
		func(name string) (io.ReadCloser, error) {
			if name != "go.mod" {
				return nil, fmt.Errorf("unexpected file")
			}
			return io.NopCloser(bytes.NewReader(modData)), nil
		},
	)
	if err != nil {
		return fmt.Errorf("capsule: hash module go.mod: %w", err)
	}

	if err := verifyGoSumEntry(
		goSum, mod.Path+" "+mod.Version, zipHash,
	); err != nil {
		return err
	}

	if err := verifyGoSumEntry(
		goSum, mod.Path+" "+mod.Version+"/go.mod", modHash,
	); err != nil {
		return err
	}

	return nil
}
