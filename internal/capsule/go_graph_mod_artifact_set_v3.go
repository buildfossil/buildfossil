package capsule

import "fmt"

// VerifyGoGraphModArtifactSetV3 verifies additional module graph
// metadata independently from the selected module artifact set.
func VerifyGoGraphModArtifactSetV3(
	files []GoGraphModFileV3,
	artifacts map[string][]byte,
	goSum []byte,
) error {
	if err := ValidateGoGraphModFilesV3(files); err != nil {
		return err
	}

	if len(artifacts) != len(files) {
		return fmt.Errorf("capsule: graph mod artifact count mismatch")
	}

	for _, file := range files {
		data, ok := artifacts[file.Artifact.Path]
		if !ok {
			return fmt.Errorf(
				"capsule: missing graph mod artifact %q",
				file.Artifact.Path,
			)
		}

		if err := VerifyGoGraphModFileV3(
			file, data, goSum,
		); err != nil {
			return fmt.Errorf(
				"capsule: verify graph mod %s@%s: %w",
				file.Path, file.Version, err,
			)
		}
	}

	return nil
}

// SplitGoArtifactsV3 separates declared selected-module artifacts
// from declared graph-only .mod artifacts. Unknown and duplicate
// artifact names are rejected.
func SplitGoArtifactsV3(
	modules []GoModuleV3,
	graphFiles []GoGraphModFileV3,
	artifacts map[string][]byte,
) (map[string][]byte, map[string][]byte, error) {
	selected := make(map[string][]byte)
	graph := make(map[string][]byte)

	selectedIdentities := make(map[string]bool, len(modules))
	for _, mod := range modules {
		identity := mod.Path + "@" + mod.Version
		if selectedIdentities[identity] {
			return nil, nil, fmt.Errorf(
				"capsule: duplicate selected module %s", identity,
			)
		}
		selectedIdentities[identity] = true
	}

	graphIdentities := make(map[string]bool, len(graphFiles))
	for _, file := range graphFiles {
		identity := file.Path + "@" + file.Version

		if selectedIdentities[identity] {
			return nil, nil, fmt.Errorf(
				"capsule: graph metadata duplicates selected module %s",
				identity,
			)
		}
		if graphIdentities[identity] {
			return nil, nil, fmt.Errorf(
				"capsule: duplicate graph module %s", identity,
			)
		}
		graphIdentities[identity] = true
	}

	expected := make(map[string]bool)

	for _, mod := range modules {
		for _, artifact := range mod.Artifacts {
			if expected[artifact.Path] {
				return nil, nil, fmt.Errorf(
					"capsule: duplicate declared artifact %q",
					artifact.Path,
				)
			}
			expected[artifact.Path] = true
			data, ok := artifacts[artifact.Path]
			if !ok {
				return nil, nil, fmt.Errorf(
					"capsule: missing artifact %q",
					artifact.Path,
				)
			}
			selected[artifact.Path] = data
		}
	}

	for _, file := range graphFiles {
		name := file.Artifact.Path
		if expected[name] {
			return nil, nil, fmt.Errorf(
				"capsule: duplicate declared artifact %q", name,
			)
		}
		expected[name] = true
		data, ok := artifacts[name]
		if !ok {
			return nil, nil, fmt.Errorf(
				"capsule: missing artifact %q", name,
			)
		}
		graph[name] = data
	}

	if len(artifacts) != len(expected) {
		return nil, nil, fmt.Errorf(
			"capsule: unexpected Go artifacts",
		)
	}

	return selected, graph, nil
}
