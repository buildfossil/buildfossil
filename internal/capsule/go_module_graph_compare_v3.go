package capsule

import "fmt"

// CompareGoModuleGraphV3 requires an exact ordered match between
// manifest modules and the externally resolved Go module graph.
//
// The main workspace module is excluded from actual.
func CompareGoModuleGraphV3(
	expected []GoModuleV3,
	actual []GoModuleIdentityV3,
) error {
	if len(expected) != len(actual) {
		return fmt.Errorf(
			"capsule: resolved Go module count mismatch: manifest=%d resolved=%d",
			len(expected), len(actual),
		)
	}

	for i := range expected {
		if i > 0 && actual[i-1].Path >= actual[i].Path {
			return fmt.Errorf("capsule: resolved Go module graph is not strictly ordered")
		}

		if expected[i].Path != actual[i].Path ||
			expected[i].Version != actual[i].Version {
			return fmt.Errorf(
				"capsule: resolved Go module mismatch at index %d: expected %s@%s, got %s@%s",
				i,
				expected[i].Path,
				expected[i].Version,
				actual[i].Path,
				actual[i].Version,
			)
		}
	}

	return nil
}
