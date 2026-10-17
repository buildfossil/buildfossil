package capsule

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	goModuleGraphTimeoutV3 = 30 * time.Second
	goModuleGraphMaxOutput = 1 << 20
)

// ListGoModuleGraphV3 determines selected Go modules without
// permitting Go module downloads.
//
// This is not a filesystem sandbox. It invokes the local Go toolchain.
func ListGoModuleGraphV3(
	workspaceRoot string,
	moduleCacheRoot string,
) ([]GoModuleIdentityV3, error) {
	goExecutable, err := ResolveGoExecutable()
	if err != nil {
		return nil, err
	}
	return ListGoModuleGraphV3WithExecutable(workspaceRoot, moduleCacheRoot, goExecutable)
}

func ListGoModuleGraphV3WithExecutable(
	workspaceRoot string,
	moduleCacheRoot string,
	goExecutable string,
) ([]GoModuleIdentityV3, error) {
	if !filepath.IsAbs(goExecutable) {
		return nil, fmt.Errorf("capsule: Go executable must be absolute")
	}

	if workspaceRoot == "" || moduleCacheRoot == "" {
		return nil, fmt.Errorf("capsule: missing workspace or module cache")
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		goModuleGraphTimeoutV3,
	)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		goExecutable,
		"list",
		"-m",
		"-json",
		"all",
	)
	cmd.Dir = workspaceRoot

	// Remove inherited values for the settings we control.
	env := make([]string, 0, len(os.Environ())+9)
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

	stdout := limitedOutputV3{limit: goModuleGraphMaxOutput}
	stderr := limitedOutputV3{limit: 64 << 10}

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf(
				"capsule: Go module graph timed out: %w",
				ctx.Err(),
			)
		}

		return nil, fmt.Errorf(
			"capsule: offline Go module graph failed: %w: %s",
			err,
			strings.TrimSpace(stderr.String()),
		)
	}

	return ParseGoModuleGraphV3(stdout.Bytes())
}
