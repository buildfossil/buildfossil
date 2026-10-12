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

const goEmbedDiscoveryTimeoutV3 = 60 * time.Second

// DiscoverGoEmbedFilesV3 finds embedded resources in packages
// selected by go build ./... for linux/amd64 and, when requested,
// darwin/arm64. The results are deduplicated.
//
// Discovery uses the local Go toolchain without network module
// downloads. The subprocess is not a filesystem or network sandbox.
func DiscoverGoEmbedFilesV3(
	workspaceRoot string,
	moduleCacheRoot string,
	sourceTarget ...GoBuildEnvironment,
) ([]string, error) {
	targets := []GoBuildEnvironment{
		{GOOS: "linux", GOARCH: "amd64"},
	}

	if len(sourceTarget) > 1 {
		return nil, fmt.Errorf("capsule: multiple source targets")
	}

	if len(sourceTarget) == 1 {
		source := sourceTarget[0]
		if !((source.GOOS == "darwin" && source.GOARCH == "arm64") ||
			(source.GOOS == "linux" && source.GOARCH == "amd64")) {
			return nil, fmt.Errorf("capsule: unsupported embed source target")
		}

		if source.GOOS != "linux" || source.GOARCH != "amd64" {
			targets = append(targets, source)
		}
	}

	seen := make(map[string]struct{})
	for _, target := range targets {
		found, err := discoverGoEmbedForTargetV3(
			workspaceRoot, moduleCacheRoot, target.GOOS, target.GOARCH,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"capsule: discover %s/%s embeds: %w",
				target.GOOS, target.GOARCH, err,
			)
		}

		for _, name := range found {
			seen[name] = struct{}{}
			if len(seen) > MaxWorkspaceV3Files {
				return nil, fmt.Errorf("capsule: too many embedded files")
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

func discoverGoEmbedForTargetV3(
	workspaceRoot string,
	moduleCacheRoot string,
	targetGOOS string,
	targetGOARCH string,
) ([]string, error) {
	if workspaceRoot == "" || moduleCacheRoot == "" {
		return nil, fmt.Errorf(
			"capsule: missing Go embed workspace or module cache",
		)
	}

	// Resolve the workspace before invoking Go. On macOS,
	// "." and /var may otherwise differ from the physical
	// paths returned in go list package metadata.
	absoluteRoot, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return nil, fmt.Errorf("capsule: resolve embed workspace: %w", err)
	}
	physicalRoot, err := filepath.EvalSymlinks(absoluteRoot)
	if err != nil {
		return nil, fmt.Errorf("capsule: canonicalize embed workspace: %w", err)
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		goEmbedDiscoveryTimeoutV3,
	)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		"go", "list", "-e", "-json", "./...",
	)
	cmd.Dir = physicalRoot

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
				"capsule: Go embed discovery timed out: %w",
				ctx.Err(),
			)
		}

		return nil, fmt.Errorf(
			"capsule: offline Go embed listing failed: %w: %s",
			err,
			strings.TrimSpace(stderr.String()),
		)
	}

	if int64(len(stdout.Bytes())) > MaxGoDependencyDiagnosticsSize {
		return nil, fmt.Errorf(
			"capsule: Go embed listing exceeds size limit",
		)
	}

	return ParseGoEmbedFilesV3(
		bytes.NewReader(stdout.Bytes()),
		physicalRoot,
	)
}
