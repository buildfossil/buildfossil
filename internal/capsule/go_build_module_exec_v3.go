package capsule

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const goBuildModuleDiscoveryTimeoutV3 = 60 * time.Second

// ListGoBuildModulesV3 finds external modules actually used by
// go build ./... without allowing network module downloads.
//
// This invokes the local Go toolchain. It is not a filesystem sandbox.
func ListGoBuildModulesV3(
	workspaceRoot string,
	moduleCacheRoot string,
	selected []GoModuleIdentityV3,
) ([]GoModuleIdentityV3, error) {
	if workspaceRoot == "" || moduleCacheRoot == "" {
		return nil, fmt.Errorf(
			"capsule: missing build workspace or module cache",
		)
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		goBuildModuleDiscoveryTimeoutV3,
	)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		"go", "list", "-deps", "-e", "-json", "./...",
	)
	cmd.Dir = workspaceRoot

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

	cmd.Env = append(
		env,
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

	stdout := limitedOutputV3{
		limit: int(MaxGoDependencyDiagnosticsSize),
	}
	stderr := limitedOutputV3{
		limit: 64 << 10,
	}

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf(
				"capsule: Go build module discovery timed out: %w",
				ctx.Err(),
			)
		}
		return nil, fmt.Errorf(
			"capsule: offline Go package listing failed: %w: %s",
			err, strings.TrimSpace(stderr.String()),
		)
	}

	if int64(len(stdout.Bytes())) > MaxGoDependencyDiagnosticsSize {
		return nil, fmt.Errorf(
			"capsule: Go package listing exceeds size limit",
		)
	}

	return SelectGoBuildModulesV3(
		bytes.NewReader(stdout.Bytes()),
		selected,
	)
}
