package docker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"
)

func TestDockerSelectedGraphCapacityV3(t *testing.T) {
	for _, count := range []int{17, 64, 65} {
		t.Run(fmt.Sprintf("modules_%d", count), func(t *testing.T) {
			expected := make([]ExpectedGoModule, 0, count)

			var output bytes.Buffer
			encoder := json.NewEncoder(&output)

			if err := encoder.Encode(map[string]any{
				"Path": "example.com/main",
				"Main": true,
			}); err != nil {
				t.Fatal(err)
			}

			for i := 0; i < count; i++ {
				mod := ExpectedGoModule{
					Path:    fmt.Sprintf("example.com/module%03d", i),
					Version: "v1.0.0",
				}
				expected = append(expected, mod)

				if err := encoder.Encode(map[string]any{
					"Path":    mod.Path,
					"Version": mod.Version,
				}); err != nil {
					t.Fatal(err)
				}
			}

			actual, parseErr := parseDockerGoModuleGraph(
				output.Bytes(),
			)

			shouldPass := count <= maxDockerSelectedGoModulesV3

			if !shouldPass {
				if parseErr == nil {
					t.Fatal("parser accepted oversized graph")
				}
				if err := compareDockerGoModuleGraph(
					expected, expected,
				); err == nil {
					t.Fatal("comparison accepted oversized graph")
				}
				return
			}

			if parseErr != nil {
				t.Fatalf("parse %d modules: %v", count, parseErr)
			}

			if err := compareDockerGoModuleGraph(
				expected, actual,
			); err != nil {
				t.Fatalf("compare %d modules: %v", count, err)
			}
		})
	}
}
