package docker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/buildfossil/buildfossil/internal/capsule"
)

func TestDockerGoModuleGraphLargeV3(t *testing.T) {
	if maxDockerSelectedGoModulesV3 != capsule.MaxGoSelectedModulesV3 {
		t.Fatal("Docker and capsule selected-module limits differ")
	}

	for _, count := range []int{64, 82, 128, 129} {
		t.Run(fmt.Sprintf("%d_modules", count), func(t *testing.T) {
			var output bytes.Buffer
			encoder := json.NewEncoder(&output)

			if err := encoder.Encode(dockerListedGoModule{
				Path: "example.com/main",
				Main: true,
			}); err != nil {
				t.Fatal(err)
			}

			expected := make([]ExpectedGoModule, 0, count)

			for i := 0; i < count; i++ {
				item := ExpectedGoModule{
					Path:    fmt.Sprintf("example.com/module%03d", i),
					Version: "v1.0.0",
				}

				expected = append(expected, item)

				if err := encoder.Encode(dockerListedGoModule{
					Path:    item.Path,
					Version: item.Version,
				}); err != nil {
					t.Fatal(err)
				}
			}

			actual, err := parseDockerGoModuleGraph(output.Bytes())

			if count > capsule.MaxGoSelectedModulesV3 {
				if err == nil ||
					!strings.Contains(err.Error(), "module graph exceeds limit") {
					t.Fatalf("oversized graph not rejected: %v", err)
				}

				if err := compareDockerGoModuleGraph(
					expected, expected,
				); err == nil {
					t.Fatal("oversized expected graph accepted")
				}

				return
			}

			if err != nil {
				t.Fatalf("valid graph rejected: %v", err)
			}

			if len(actual) != count {
				t.Fatalf("got %d modules, want %d", len(actual), count)
			}

			if err := compareDockerGoModuleGraph(
				expected, actual,
			); err != nil {
				t.Fatalf("matching graph rejected: %v", err)
			}

			if count > 0 {
				different := append([]ExpectedGoModule(nil), actual...)
				different[0].Version = "v1.0.1"

				if err := compareDockerGoModuleGraph(
					expected, different,
				); err == nil {
					t.Fatal("mismatched graph accepted")
				}
			}
		})
	}
}
