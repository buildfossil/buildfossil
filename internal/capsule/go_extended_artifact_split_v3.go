package capsule

import "fmt"

// SplitExtendedGoArtifactsV3 separates full module artifacts,
// additional graph .mod artifacts, and metadata-only .info files.
func SplitExtendedGoArtifactsV3(
	manifest Manifest,
	artifacts map[string][]byte,
) (
	map[string][]byte,
	map[string][]byte,
	map[string][]byte,
	error,
) {
	full := make(map[string][]byte)
	graphMod := make(map[string][]byte)
	graphInfo := make(map[string][]byte)
	expected := make(map[string]bool)

	take := func(name string, target map[string][]byte) error {
		if expected[name] {
			return fmt.Errorf(
				"capsule: duplicate declared artifact %q", name,
			)
		}
		expected[name] = true

		data, exists := artifacts[name]
		if !exists {
			return fmt.Errorf(
				"capsule: missing declared artifact %q", name,
			)
		}

		target[name] = data
		return nil
	}

	for _, mod := range manifest.GoModules {
		for _, artifact := range mod.Artifacts {
			if err := take(artifact.Path, full); err != nil {
				return nil, nil, nil, err
			}
		}
	}

	fullIdentities := make(map[string]bool)
	for _, mod := range manifest.GoModules {
		fullIdentities[mod.Path+"@"+mod.Version] = true
	}

	for _, file := range manifest.GoGraphModFiles {
		if fullIdentities[file.Path+"@"+file.Version] {
			return nil, nil, nil, fmt.Errorf(
				"capsule: graph mod duplicates full module",
			)
		}

		if err := take(file.Artifact.Path, graphMod); err != nil {
			return nil, nil, nil, err
		}
	}

	for _, file := range manifest.GoGraphInfoFiles {
		if err := take(file.Artifact.Path, graphInfo); err != nil {
			return nil, nil, nil, err
		}
	}

	if len(artifacts) != len(expected) {
		return nil, nil, nil, fmt.Errorf(
			"capsule: unexpected Go artifacts",
		)
	}

	return full, graphMod, graphInfo, nil
}
