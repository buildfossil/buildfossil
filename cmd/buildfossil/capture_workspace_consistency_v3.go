package main

import (
	"bytes"
	"fmt"

	"github.com/buildfossil/buildfossil/internal/capsule"
)

// compareCapturedWorkspaceV3 checks that the included files have the
// same paths, permissions and content in both snapshots.
//
// This detects changes between reads; it is not an atomic filesystem snapshot.
func compareCapturedWorkspaceV3(
	before []capsule.WorkspaceFile,
	after []capsule.WorkspaceFile,
) error {
	if len(before) != len(after) {
		return fmt.Errorf("workspace file count changed during capture")
	}

	for i := range before {
		a := before[i]
		b := after[i]

		if a.Path != b.Path {
			return fmt.Errorf("workspace file list changed during capture")
		}

		if a.Mode != b.Mode {
			return fmt.Errorf(
				"workspace file mode changed during capture: %s",
				a.Path,
			)
		}

		if !bytes.Equal(a.Data, b.Data) {
			return fmt.Errorf(
				"workspace file changed during capture: %s",
				a.Path,
			)
		}
	}

	return nil
}

// verifyCapturedWorkspaceV3 rereads included workspace files and
// rejects changes relative to the previously captured snapshot.
func verifyCapturedWorkspaceV3(
	workspaceRoot string,
	includes []string,
	expected []capsule.WorkspaceFile,
) error {
	current, err := capsule.ReadWorkspaceFilesV3(
		workspaceRoot,
		includes,
	)
	if err != nil {
		return fmt.Errorf("reread workspace: %w", err)
	}

	if err := compareCapturedWorkspaceV3(expected, current); err != nil {
		return fmt.Errorf("workspace verification: %w", err)
	}

	return nil
}
