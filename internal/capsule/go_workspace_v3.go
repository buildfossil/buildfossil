package capsule

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ValidateGoWorkspaceV3 rejects an active go.work configuration.
// An empty effective GOWORK means module mode is in use.
func ValidateGoWorkspaceV3(workingDir string) error {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "env", "GOWORK")
	cmd.Dir = workingDir

	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf(
			"capsule: inspect Go workspace: %w",
			err,
		)
	}

	goWork := strings.TrimSpace(string(output))

	if goWork != "" && goWork != "off" {
		return fmt.Errorf(
			"capsule: v3 capture does not support active go.work: %s",
			goWork,
		)
	}

	return nil
}
