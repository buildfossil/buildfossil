package capsule

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const goSourceDiscoveryTimeoutV3 = 60 * time.Second

// DiscoverGoSourceFilesV3 discovers selected Go sources for Linux
// AMD64 and, when different, the supported source platform.
//
// Discovery does not download modules. It is not a sandbox:
// only invoke it on workspaces trusted by the user.
func DiscoverGoSourceFilesV3(
	workspaceRoot string,
	moduleCacheRoot string,
	sourceTarget ...GoBuildEnvironment,
) ([]string, error) {
	goExecutable, err := ResolveGoExecutable()
	if err != nil {
		return nil, err
	}
	return DiscoverGoSourceFilesV3WithExecutable(
		workspaceRoot, moduleCacheRoot, goExecutable, sourceTarget...,
	)
}

// DiscoverGoSourceFilesV3WithExecutable runs discovery using the selected Go toolchain.
func DiscoverGoSourceFilesV3WithExecutable(
	workspaceRoot string,
	moduleCacheRoot string,
	goExecutable string,
	sourceTarget ...GoBuildEnvironment,
) ([]string, error) {
	if !filepath.IsAbs(goExecutable) {
		return nil, fmt.Errorf("capsule: Go executable must be absolute")
	}

	if workspaceRoot == "" || moduleCacheRoot == "" {
		return nil, fmt.Errorf("capsule: missing Go source discovery input")
	}

	if len(sourceTarget) > 1 {
		return nil, fmt.Errorf("capsule: multiple source targets")
	}

	targets := []GoBuildEnvironment{
		{GOOS: "linux", GOARCH: "amd64"},
	}

	if len(sourceTarget) == 1 {
		source := sourceTarget[0]

		if !((source.GOOS == "linux" && source.GOARCH == "amd64") ||
			(source.GOOS == "darwin" && source.GOARCH == "arm64")) {
			return nil, fmt.Errorf(
				"capsule: unsupported Go source target %s/%s",
				source.GOOS, source.GOARCH,
			)
		}

		if source.GOOS != "linux" || source.GOARCH != "amd64" {
			targets = append(targets, source)
		}
	}

	absoluteRoot, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return nil, fmt.Errorf("capsule: resolve source workspace: %w", err)
	}

	physicalRoot, err := filepath.EvalSymlinks(absoluteRoot)
	if err != nil {
		return nil, fmt.Errorf("capsule: canonicalize source workspace: %w", err)
	}

	seen := make(map[string]struct{})

	for _, target := range targets {
		found, err := discoverGoSourcesForTargetV3(
			goExecutable,
			physicalRoot,
			moduleCacheRoot,
			target.GOOS,
			target.GOARCH,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"capsule: discover %s/%s Go sources: %w",
				target.GOOS, target.GOARCH, err,
			)
		}

		for _, name := range found {
			seen[name] = struct{}{}

			if len(seen) > MaxWorkspaceV3Files {
				return nil, fmt.Errorf(
					"capsule: too many discovered Go source files",
				)
			}
		}
	}

	result := make([]string, 0, len(seen))
	for name := range seen {
		result = append(result, name)
	}

	sort.Strings(result)
	return result, nil
}

func discoverGoSourcesForTargetV3(
	goExecutable string,
	workspaceRoot string,
	moduleCacheRoot string,
	targetGOOS string,
	targetGOARCH string,
) ([]string, error) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		goSourceDiscoveryTimeoutV3,
	)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		goExecutable, "list", "-e", "-json", "./...",
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
		"GOOS="+targetGOOS,
		"GOARCH="+targetGOARCH,
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
				"capsule: Go source discovery timed out: %w",
				ctx.Err(),
			)
		}

		return nil, fmt.Errorf(
			"capsule: offline Go source listing failed: %w: %s",
			err,
			strings.TrimSpace(stderr.String()),
		)
	}

	if int64(len(stdout.Bytes())) > MaxGoDependencyDiagnosticsSize {
		return nil, fmt.Errorf(
			"capsule: Go source listing exceeds size limit",
		)
	}

	return ParseGoSourceFilesV3(
		bytes.NewReader(stdout.Bytes()),
		workspaceRoot,
	)
}
