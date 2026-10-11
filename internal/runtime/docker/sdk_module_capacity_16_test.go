package docker

import (
	"fmt"
	"testing"
)

func TestDockerModuleGraphSupports16Modules(t *testing.T) {
	expected := make([]ExpectedGoModule, 16)

	for i := range expected {
		expected[i] = ExpectedGoModule{
			Path:    fmt.Sprintf("example.com/mod%04d", i),
			Version: "v1.0.0",
		}
	}

	if err := compareDockerGoModuleGraph(expected, expected); err != nil {
		t.Fatalf("16-module graph rejected: %v", err)
	}

	seventeen := append(
		append([]ExpectedGoModule{}, expected...),
		ExpectedGoModule{
			Path:    "example.com/mod0016",
			Version: "v1.0.0",
		},
	)

	if err := compareDockerGoModuleGraph(seventeen, seventeen); err != nil {
		t.Fatalf("17-module selected graph rejected: %v", err)
	}
}

func TestDockerModuleGraph16RejectsVersionMismatch(t *testing.T) {
	expected := make([]ExpectedGoModule, 16)

	for i := range expected {
		expected[i] = ExpectedGoModule{
			Path:    fmt.Sprintf("example.com/mod%04d", i),
			Version: "v1.0.0",
		}
	}

	actual := append([]ExpectedGoModule(nil), expected...)
	actual[15].Version = "v1.1.0"

	if err := compareDockerGoModuleGraph(expected, actual); err == nil {
		t.Fatal("16-module graph with modified dependency accepted")
	}
}
