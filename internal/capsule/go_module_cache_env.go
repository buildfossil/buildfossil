package capsule

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// InspectGoModuleCacheWithExecutable resolves GOMODCACHE using the
// explicitly selected Go executable without automatic toolchain switching.
func InspectGoModuleCacheWithExecutable(
	workingDir string,
	goExecutable string,
) (string, error) {
	if !filepath.IsAbs(goExecutable) {
		return "", fmt.Errorf("capsule: Go executable must be absolute")
	}

	ctx, cancel := context.WithTimeout(
		context.Background(), 5*time.Second,
	)
	defer cancel()

	cmd := exec.CommandContext(ctx, goExecutable, "env", "GOMODCACHE")
	cmd.Dir = workingDir
	cmd.Env = LocalGoToolchainEnv()

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("capsule: resolve GOMODCACHE: %w", err)
	}

	moduleCacheRoot := strings.TrimSpace(string(output))
	if moduleCacheRoot == "" {
		return "", fmt.Errorf("capsule: empty GOMODCACHE")
	}

	return moduleCacheRoot, nil
}
