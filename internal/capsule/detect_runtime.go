package capsule

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// DetectRuntime identifies supported toolchains for capture.
// Unknown commands return nil, preserving legacy behavior.
func DetectRuntime(argv []string, workingDir string) *Runtime {
	goExecutable, err := ResolveGoExecutable()
	if err != nil {
		return nil
	}

	return DetectRuntimeWithExecutable(argv, workingDir, goExecutable)
}

// DetectRuntimeWithExecutable checks the runtime using an explicitly
// selected Go executable. It does not change supported Go versions.
func DetectRuntimeWithExecutable(
	argv []string,
	workingDir string,
	goExecutable string,
) *Runtime {
	if len(argv) != 3 ||
		argv[0] != "go" ||
		argv[1] != "build" ||
		argv[2] != "./..." {
		return nil
	}

	if !filepath.IsAbs(goExecutable) {
		return nil
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	cmd := exec.CommandContext(ctx, goExecutable, "version")
	cmd.Dir = workingDir
	cmd.Env = LocalGoToolchainEnv()

	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	fields := strings.Fields(string(output))
	if len(fields) < 3 || fields[0] != "go" || fields[1] != "version" {
		return nil
	}

	version, ok := strings.CutPrefix(fields[2], "go")
	if !ok || !IsSupportedGoVersion(version) {
		return nil
	}

	return &Runtime{
		Kind:    "go",
		Version: version,
	}
}
