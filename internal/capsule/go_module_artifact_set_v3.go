package capsule

import "fmt"

// VerifyGoModuleArtifactSetV3 validates the entire artifact set.
// It rejects missing, extra, and incorrectly indexed artifacts.
func VerifyGoModuleArtifactSetV3(
	modules []GoModuleV3,
	artifacts map[string][]byte,
	goSum []byte,
) error {
	if err := ValidateGoModulesV3(modules); err != nil {
		return err
	}

	if len(artifacts) != len(modules)*MaxGoModuleArtifactsV3 {
		return fmt.Errorf("capsule: Go artifact set count mismatch")
	}

	if len(modules) == 0 {
		return nil
	}

	var totalSize int64

	for i, mod := range modules {
		subset := make(map[string][]byte, MaxGoModuleArtifactsV3)

		for _, metadata := range mod.Artifacts {
			data, ok := artifacts[metadata.Path]
			if !ok {
				return fmt.Errorf(
					"capsule: missing Go artifact %q",
					metadata.Path,
				)
			}

			if int64(len(data)) > MaxGoModuleTotalSizeV3-totalSize {
				return fmt.Errorf("capsule: Go artifact size limit exceeded")
			}
			totalSize += int64(len(data))

			subset[metadata.Path] = data
		}

		if err := verifyGoModuleArtifactsAtIndexV3(
			mod,
			subset,
			goSum,
			i+1,
		); err != nil {
			return fmt.Errorf(
				"capsule: verify Go module %d (%s): %w",
				i+1,
				mod.Path,
				err,
			)
		}
	}

	// Count equality plus the validated, unique manifest paths
	// ensure no undeclared artifacts remain.
	return nil
}
