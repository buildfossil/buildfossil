package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/moby/moby/client"
	"golang.org/x/mod/module"
)

// ExpectedGoModule identifies a selected external Go module.
type ExpectedGoModule struct {
	Path    string
	Version string
}

// Keep in sync with capsule.MaxGoSelectedModulesV3.
const maxDockerSelectedGoModulesV3 = 128

type dockerListedGoModule struct {
	Path    string          `json:"Path"`
	Version string          `json:"Version"`
	Main    bool            `json:"Main"`
	Replace json.RawMessage `json:"Replace"`
	Error   json.RawMessage `json:"Error"`
}

// parseDockerGoModuleGraph parses the bounded JSON output of
// go list -m -json all, excluding the main module.
func parseDockerGoModuleGraph(data []byte) ([]ExpectedGoModule, error) {
	if len(data) == 0 || len(data) > 1<<20 {
		return nil, fmt.Errorf("docker SDK: invalid module graph size")
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	var modules []ExpectedGoModule
	seen := make(map[string]bool)
	mainCount := 0

	for {
		var item dockerListedGoModule

		err := decoder.Decode(&item)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("docker SDK: decode module graph: %w", err)
		}

		if len(item.Replace) != 0 && string(item.Replace) != "null" {
			return nil, fmt.Errorf("docker SDK: module replacement rejected")
		}
		if len(item.Error) != 0 && string(item.Error) != "null" {
			return nil, fmt.Errorf("docker SDK: module graph error")
		}

		if item.Main {
			mainCount++
			if item.Path == "" || item.Version != "" {
				return nil, fmt.Errorf("docker SDK: invalid main module")
			}
			continue
		}

		if err := module.Check(item.Path, item.Version); err != nil {
			return nil, fmt.Errorf("docker SDK: invalid module identity: %w", err)
		}

		if seen[item.Path] {
			return nil, fmt.Errorf("docker SDK: duplicate module path")
		}
		seen[item.Path] = true

		modules = append(modules, ExpectedGoModule{
			Path: item.Path, Version: item.Version,
		})

		if len(modules) > maxDockerSelectedGoModulesV3 {
			return nil, fmt.Errorf("docker SDK: module graph exceeds limit")
		}
	}

	if mainCount != 1 {
		return nil, fmt.Errorf("docker SDK: expected exactly one main module")
	}

	sort.Slice(modules, func(i, j int) bool {
		return modules[i].Path < modules[j].Path
	})

	return modules, nil
}

func compareDockerGoModuleGraph(
	expected []ExpectedGoModule,
	actual []ExpectedGoModule,
) error {
	if len(expected) > maxDockerSelectedGoModulesV3 || len(expected) != len(actual) {
		return fmt.Errorf("docker SDK: resolved Go module count mismatch")
	}

	for i := range expected {
		if err := module.Check(expected[i].Path, expected[i].Version); err != nil {
			return fmt.Errorf("docker SDK: invalid expected module: %w", err)
		}

		if i > 0 && expected[i-1].Path >= expected[i].Path {
			return fmt.Errorf("docker SDK: expected modules are not strictly sorted")
		}

		if expected[i] != actual[i] {
			return fmt.Errorf(
				"docker SDK: resolved module mismatch at index %d: expected %s@%s, got %s@%s",
				i,
				expected[i].Path, expected[i].Version,
				actual[i].Path, actual[i].Version,
			)
		}
	}

	return nil
}

// verifyOfflineGoModuleGraph checks the complete selected graph
// inside the already prepared execution container.
func verifyOfflineGoModuleGraph(
	ctx context.Context,
	dockerClient *client.Client,
	containerID string,
	user string,
	expected []ExpectedGoModule,
) error {
	code, stdout, stderr, err := executeInContainer(
		ctx,
		dockerClient,
		containerID,
		user,
		[]string{"go", "list", "-m", "-json", "all"},
	)
	if err != nil {
		return fmt.Errorf("docker SDK: offline graph command: %w", err)
	}
	if code != 0 {
		return fmt.Errorf(
			"docker SDK: offline graph resolution failed (exit %d): %s",
			code, string(stderr),
		)
	}

	actual, err := parseDockerGoModuleGraph(stdout)
	if err != nil {
		return err
	}

	return compareDockerGoModuleGraph(expected, actual)
}
