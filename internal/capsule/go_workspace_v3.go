package capsule

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ValidateGoWorkspaceV3 rejects an active go.work configuration.
// An empty effective GOWORK means module mode is in use.
func ValidateGoWorkspaceV3(workingDir string) error {
	goExecutable, err := ResolveGoExecutable()
	if err != nil {
		return err
	}
	return ValidateGoWorkspaceV3WithExecutable(
		workingDir, goExecutable,
	)
}

// ValidateGoWorkspaceV3WithExecutable checks GOWORK using the selected
// Go executable without automatic toolchain switching.
func ValidateGoWorkspaceV3WithExecutable(
	workingDir string,
	goExecutable string,
) error {
	if !filepath.IsAbs(goExecutable) {
		return fmt.Errorf("capsule: Go executable must be absolute")
	}
	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	cmd := exec.CommandContext(ctx, goExecutable, "env", "GOWORK")
	cmd.Dir = workingDir
	cmd.Env = LocalGoToolchainEnv()

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
