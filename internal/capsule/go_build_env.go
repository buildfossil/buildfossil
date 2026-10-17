package capsule

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type GoBuildEnvironment struct {
	GOOS       string `json:"goos"`
	GOARCH     string `json:"goarch"`
	CGOEnabled string `json:"cgo_enabled"`
	GOFLAGS    string `json:"goflags"`
}

// InspectGoBuildEnvironment reads the effective Go configuration.
// It does not modify the user's environment.
func InspectGoBuildEnvironment(workingDir string) (
	GoBuildEnvironment,
	error,
) {
	goExecutable, err := ResolveGoExecutable()
	if err != nil {
		return GoBuildEnvironment{}, err
	}

	return InspectGoBuildEnvironmentWithExecutable(
		workingDir, goExecutable,
	)
}

// InspectGoBuildEnvironmentWithExecutable reads the Go environment
// through an explicitly selected executable with GOTOOLCHAIN=local.
func InspectGoBuildEnvironmentWithExecutable(
	workingDir string,
	goExecutable string,
) (GoBuildEnvironment, error) {
	if !filepath.IsAbs(goExecutable) {
		return GoBuildEnvironment{}, fmt.Errorf(
			"inspect Go environment: executable must be absolute",
		)
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		goExecutable,
		"env",
		"GOOS",
		"GOARCH",
		"CGO_ENABLED",
		"GOFLAGS",
	)

	cmd.Dir = workingDir
	cmd.Env = LocalGoToolchainEnv()

	output, err := cmd.Output()
	if err != nil {
		return GoBuildEnvironment{}, fmt.Errorf(
			"inspect Go environment: %w",
			err,
		)
	}

	lines := strings.Split(
		strings.TrimSuffix(string(output), "\n"),
		"\n",
	)

	if len(lines) != 4 {
		return GoBuildEnvironment{}, fmt.Errorf(
			"unexpected Go environment output",
		)
	}

	return GoBuildEnvironment{
		GOOS:       lines[0],
		GOARCH:     lines[1],
		CGOEnabled: lines[2],
		GOFLAGS:    lines[3],
	}, nil
}

// ValidatePortableGoBuild checks the initial portability policy.
func (e GoBuildEnvironment) ValidatePortableGoBuild() error {
	if !((e.GOOS == "darwin" && e.GOARCH == "arm64") ||
		(e.GOOS == "linux" && e.GOARCH == "amd64")) {
		return fmt.Errorf(
			"unsupported portable Go source platform %s/%s",
			e.GOOS,
			e.GOARCH,
		)
	}

	if e.CGOEnabled != "0" {
		return fmt.Errorf(
			"portable Go replay requires CGO_ENABLED=0",
		)
	}

	if e.GOFLAGS != "" {
		return fmt.Errorf(
			"portable Go replay requires empty GOFLAGS",
		)
	}

	return nil
}
