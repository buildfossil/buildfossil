package capsule

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// DetectRuntime identifies supported toolchains for capture.
// Unknown commands return nil, preserving legacy behavior.
func DetectRuntime(argv []string, workingDir string) *Runtime {
	if len(argv) != 3 ||
		argv[0] != "go" ||
		argv[1] != "build" ||
		argv[2] != "./..." {
		return nil
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "version")
	cmd.Dir = workingDir

	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	fields := strings.Fields(string(output))

	if len(fields) < 3 || fields[2] != "go"+SupportedGoVersion {
		return nil
	}

	return &Runtime{
		Kind:    "go",
		Version: SupportedGoVersion,
	}
}
