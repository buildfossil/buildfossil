package docker

import (
	"testing"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

func TestDockerRuntimeGuardsExactVersions(t *testing.T) {
	tests := []struct {
		version string
		valid   bool
	}{
		{"1.26.6", true},
		{"1.25.14", true},
		{"1.26.5", false},
		{"1.25.13", false},
		{"1.25.0", false},
		{"1.27.0", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			runtime := &RuntimeSpec{
				Kind:    "go",
				Version: tt.version,
			}

			if got := isSupportedGoRuntime(runtime); got != tt.valid {
				t.Fatalf(
					"runtime guard version %q = %v, want %v",
					tt.version, got, tt.valid,
				)
			}

			options := &client.ContainerCreateOptions{
				Config: &container.Config{},
			}

			err := configureGoTargetPlatform(
				options, runtime, "linux", "amd64",
			)

			if (err == nil) != tt.valid {
				t.Fatalf(
					"platform guard version %q: err=%v, valid=%v",
					tt.version, err, tt.valid,
				)
			}
		})
	}
}

func TestDockerRuntimeGuardRejectsNilAndWrongKind(t *testing.T) {
	if isSupportedGoRuntime(nil) {
		t.Fatal("nil runtime accepted")
	}

	if isSupportedGoRuntime(&RuntimeSpec{
		Kind: "python", Version: "1.25.14",
	}) {
		t.Fatal("unsupported runtime kind accepted")
	}
}
