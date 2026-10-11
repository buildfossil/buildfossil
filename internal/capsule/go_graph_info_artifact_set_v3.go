package capsule

import "fmt"

// VerifyGoGraphInfoArtifactSetV3 verifies the complete set of
// metadata-only .info files declared by a manifest.
//
// The caller must separately validate the manifest's selected
// graph and full ZIP module lists.
func VerifyGoGraphInfoArtifactSetV3(
	files []GoGraphInfoFileV3,
	artifacts map[string][]byte,
) error {
	if len(files) > MaxGoGraphInfoFilesV3 {
		return fmt.Errorf("capsule: too many graph info files")
	}

	if len(artifacts) != len(files) {
		return fmt.Errorf(
			"capsule: graph info artifact count mismatch",
		)
	}

	expected := make(map[string]bool, len(files))

	for _, file := range files {
		name := file.Artifact.Path

		if expected[name] {
			return fmt.Errorf(
				"capsule: duplicate graph info artifact %q", name,
			)
		}

		expected[name] = true

		data, ok := artifacts[name]
		if !ok {
			return fmt.Errorf(
				"capsule: missing graph info artifact %q", name,
			)
		}

		if err := VerifyGoGraphInfoFileV3(file, data); err != nil {
			return fmt.Errorf(
				"capsule: verify graph info %s@%s: %w",
				file.Path, file.Version, err,
			)
		}
	}

	for name := range artifacts {
		if !expected[name] {
			return fmt.Errorf(
				"capsule: unexpected graph info artifact %q", name,
			)
		}
	}

	return nil
}
