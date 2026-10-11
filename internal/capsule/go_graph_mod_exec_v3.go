package capsule

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// ListGoGraphModRequirementsV3 resolves additional graph-only
// module versions without permitting network downloads.
//
// This invokes the local Go toolchain, not a filesystem sandbox.
func ListGoGraphModRequirementsV3(
	workspaceRoot string,
	moduleCacheRoot string,
	mainModulePath string,
	selected []GoModuleIdentityV3,
) ([]GoModuleIdentityV3, error) {
	if workspaceRoot == "" || moduleCacheRoot == "" {
		return nil, fmt.Errorf("capsule: missing workspace or module cache")
	}

	ctx, cancel := context.WithTimeout(
		context.Background(), 30*time.Second,
	)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "mod", "graph")
	cmd.Dir = workspaceRoot

	env := make([]string, 0, len(os.Environ())+10)
	for _, value := range os.Environ() {
		key, _, _ := strings.Cut(value, "=")

		switch key {
		case "GOOS", "GOARCH", "CGO_ENABLED",
			"GOFLAGS", "GOWORK", "GOTOOLCHAIN",
			"GOPROXY", "GOSUMDB", "GOMODCACHE":
			continue
		}

		env = append(env, value)
	}

	cmd.Env = append(env,
		"GOOS=linux",
		"GOARCH=amd64",
		"CGO_ENABLED=0",
		"GOFLAGS=-mod=readonly",
		"GOWORK=off",
		"GOTOOLCHAIN=local",
		"GOPROXY=off",
		"GOSUMDB=off",
		"GOMODCACHE="+moduleCacheRoot,
	)

	stdout := limitedOutputV3{limit: 1 << 20}
	stderr := limitedOutputV3{limit: 64 << 10}

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf(
				"capsule: offline Go graph timed out: %w",
				ctx.Err(),
			)
		}

		return nil, fmt.Errorf(
			"capsule: offline go mod graph failed: %w: %s",
			err,
			strings.TrimSpace(stderr.String()),
		)
	}

	return ParseGoGraphModRequirementsV3(stdout.Bytes(), mainModulePath, selected)
}
