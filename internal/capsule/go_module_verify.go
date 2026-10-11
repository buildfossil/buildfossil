package capsule

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
	"golang.org/x/mod/sumdb/dirhash"
	modzip "golang.org/x/mod/zip"
)

// VerifyGoModuleArtifactsV3 preserves the single-module API.
func VerifyGoModuleArtifactsV3(
	mod GoModuleV3,
	artifacts map[string][]byte,
	goSum []byte,
) error {
	return verifyGoModuleArtifactsAtIndexV3(
		mod, artifacts, goSum, 1,
	)
}

func verifyGoModuleArtifactsAtIndexV3(
	mod GoModuleV3,
	artifacts map[string][]byte,
	goSum []byte,
	index int,
) error {
	if index < 1 || index > MaxGoModulesV3 {
		return fmt.Errorf("capsule: invalid Go module index")
	}

	if err := module.Check(mod.Path, mod.Version); err != nil {
		return fmt.Errorf("capsule: invalid Go module identity: %w", err)
	}

	prefix := fmt.Sprintf("module-%04d", index)

	if len(mod.Artifacts) != MaxGoModuleArtifactsV3 ||
		len(artifacts) != MaxGoModuleArtifactsV3 {
		return fmt.Errorf("capsule: invalid Go artifact count")
	}

	expected := map[string]bool{
		prefix + ".zip":  false,
		prefix + ".mod":  false,
		prefix + ".info": false,
	}

	var totalSize int64

	for _, metadata := range mod.Artifacts {
		seen, ok := expected[metadata.Path]
		if !ok || seen {
			return fmt.Errorf("capsule: invalid Go artifact path %q", metadata.Path)
		}
		expected[metadata.Path] = true

		if metadata.Size < 0 ||
			metadata.Size > MaxGoModuleTotalSizeV3-totalSize {
			return fmt.Errorf("capsule: Go artifact size limit exceeded")
		}
		totalSize += metadata.Size

		data, ok := artifacts[metadata.Path]
		if !ok {
			return fmt.Errorf("capsule: missing Go artifact %q", metadata.Path)
		}

		if int64(len(data)) != metadata.Size {
			return fmt.Errorf("capsule: Go artifact size mismatch %q", metadata.Path)
		}

		if SHA256(data) != metadata.SHA256 {
			return fmt.Errorf("capsule: Go artifact SHA-256 mismatch %q", metadata.Path)
		}
	}

	for name := range artifacts {
		if _, ok := expected[name]; !ok {
			return fmt.Errorf("capsule: unexpected Go artifact %q", name)
		}
	}

	modData := artifacts[prefix+".mod"]
	infoData := artifacts[prefix+".info"]
	zipData := artifacts[prefix+".zip"]

	parsed, err := modfile.Parse("go.mod", modData, nil)
	if err != nil {
		return fmt.Errorf("capsule: invalid module go.mod: %w", err)
	}

	if parsed.Module == nil ||
		parsed.Module.Mod.Path != mod.Path {
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

	// Check the module ZIP on disk using the same x/mod package
	// that defines the Go module archive constraints.
	tmpDir, err := os.MkdirTemp("", "buildfossil-module-verify-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	zipPath := filepath.Join(tmpDir, "module.zip")
	if err := os.WriteFile(zipPath, zipData, 0600); err != nil {
		return err
	}

	identity := module.Version{
		Path:    mod.Path,
		Version: mod.Version,
	}

	if _, err := modzip.CheckZip(identity, zipPath); err != nil {
		return fmt.Errorf("capsule: invalid Go module ZIP: %w", err)
	}

	if err := verifyGoModuleZipContents(zipPath); err != nil {
		return err
	}

	if err := verifyEmbeddedGoMod(zipPath, mod, modData); err != nil {
		return err
	}

	zipHash, err := dirhash.HashZip(zipPath, dirhash.Hash1)
	if err != nil {
		return fmt.Errorf("capsule: hash Go module ZIP: %w", err)
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
		goSum,
		mod.Path+" "+mod.Version,
		zipHash,
	); err != nil {
		return err
	}

	if err := verifyGoSumEntry(
		goSum,
		mod.Path+" "+mod.Version+"/go.mod",
		modHash,
	); err != nil {
		return err
	}

	return nil
}

func verifyEmbeddedGoMod(
	zipPath string,
	mod GoModuleV3,
	expected []byte,
) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer reader.Close()

	name := mod.Path + "@" + mod.Version + "/go.mod"
	found := false

	for _, file := range reader.File {
		if file.Name != name {
			continue
		}

		if found {
			return fmt.Errorf("capsule: duplicate module go.mod")
		}
		found = true

		if file.UncompressedSize64 != uint64(len(expected)) {
			return fmt.Errorf("capsule: embedded go.mod size mismatch")
		}

		r, err := file.Open()
		if err != nil {
			return err
		}

		data, readErr := io.ReadAll(
			io.LimitReader(r, int64(len(expected))+1),
		)
		closeErr := r.Close()

		if readErr != nil {
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}

		if !bytes.Equal(data, expected) {
			return fmt.Errorf("capsule: embedded go.mod mismatch")
		}
	}

	// Legacy Go modules may not contain go.mod inside their ZIP.
	// The separate .mod artifact and its go.sum checksum are
	// verified by verifyGoModuleArtifactsAtIndexV3.
	return nil
}

func verifyGoSumEntry(
	goSum []byte,
	identity string,
	expectedHash string,
) error {
	found := false

	for _, line := range strings.Split(string(goSum), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}

		if fields[0] != strings.Fields(identity)[0] {
			continue
		}

		parts := strings.Fields(identity)
		if len(fields) < 2 || fields[1] != parts[1] {
			continue
		}

		if len(fields) != 3 {
			return fmt.Errorf("capsule: invalid go.sum entry")
		}

		if found {
			return fmt.Errorf("capsule: duplicate go.sum entry")
		}
		found = true

		if fields[2] != expectedHash {
			return fmt.Errorf("capsule: Go module checksum mismatch")
		}
	}

	if !found {
		return fmt.Errorf(
			"capsule: missing go.sum entry for %s",
			identity,
		)
	}

	return nil
}

func verifyGoModuleZipContents(zipPath string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("capsule: open Go module ZIP: %w", err)
	}
	defer reader.Close()

	var declaredTotal int64

	for _, file := range reader.File {
		if !file.Mode().IsRegular() {
			return fmt.Errorf(
				"capsule: unsupported Go module ZIP entry type: %q",
				file.Name,
			)
		}

		if file.UncompressedSize64 > uint64(
			MaxGoModuleUnpackedSizeV3-declaredTotal,
		) {
			return fmt.Errorf(
				"capsule: Go module ZIP unpacked size exceeded",
			)
		}

		declaredTotal += int64(file.UncompressedSize64)
	}

	var totalSize int64

	for _, file := range reader.File {
		mode := file.Mode()

		if !mode.IsRegular() {
			return fmt.Errorf(
				"capsule: unsupported Go module ZIP entry type: %q",
				file.Name,
			)
		}

		if file.UncompressedSize64 > uint64(
			MaxGoModuleUnpackedSizeV3-totalSize,
		) {
			return fmt.Errorf(
				"capsule: Go module ZIP unpacked size exceeded",
			)
		}

		r, err := file.Open()
		if err != nil {
			return fmt.Errorf(
				"capsule: open ZIP entry %q: %w",
				file.Name,
				err,
			)
		}

		n, copyErr := io.Copy(
			io.Discard,
			io.LimitReader(
				r,
				MaxGoModuleUnpackedSizeV3-totalSize+1,
			),
		)
		closeErr := r.Close()

		if copyErr != nil {
			return fmt.Errorf(
				"capsule: read ZIP entry %q: %w",
				file.Name,
				copyErr,
			)
		}

		if closeErr != nil {
			return fmt.Errorf(
				"capsule: close ZIP entry %q: %w",
				file.Name,
				closeErr,
			)
		}

		if n != int64(file.UncompressedSize64) {
			return fmt.Errorf(
				"capsule: ZIP entry size mismatch: %q",
				file.Name,
			)
		}

		totalSize += n
	}

	return nil
}
