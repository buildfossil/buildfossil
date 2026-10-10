package replay

import (
	"context"
	"reflect"
	"testing"

	"github.com/buildfossil/buildfossil/internal/capsule"
)

func TestGoDependencyProbeArgs(t *testing.T) {
	want := []string{
		"go", "list", "-deps", "-e", "-json", "./...",
	}

	got := goDependencyProbeArgs()

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %v, want %v", got, want)
	}
}

func TestShouldProbeGoDependencies(t *testing.T) {
	tests := []struct {
		name string
		m    capsule.Manifest
		want bool
	}{
		{
			name: "supported Go runtime",
			m: capsule.Manifest{
				Runtime: &capsule.Runtime{
					Kind:    "go",
					Version: capsule.SupportedGoVersion,
				},
				GoBuildEnv: &capsule.GoBuildEnvironment{
					GOOS:       "darwin",
					GOARCH:     "arm64",
					CGOEnabled: "0",
				},
			},
			want: true,
		},
		{
			name: "no runtime",
		},
		{
			name: "no Go build environment",
			m: capsule.Manifest{
				Runtime: &capsule.Runtime{
					Kind:    "go",
					Version: capsule.SupportedGoVersion,
				},
			},
		},
		{
			name: "unsupported version",
			m: capsule.Manifest{
				Runtime: &capsule.Runtime{
					Kind:    "go",
					Version: "0.0.0",
				},
				GoBuildEnv: &capsule.GoBuildEnvironment{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldProbeGoDependencies(tt.m)

			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGoDependencyProbeCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	manifest := capsule.Manifest{
		SchemaVersion: capsule.SchemaVersionV2,
		Runtime: &capsule.Runtime{
			Kind:    "go",
			Version: capsule.SupportedGoVersion,
		},
		GoBuildEnv: &capsule.GoBuildEnvironment{
			GOOS:       "linux",
			GOARCH:     "amd64",
			CGOEnabled: "0",
		},
		Platform: capsule.Platform{
			OS:           "linux",
			Architecture: "amd64",
		},
	}

	status := probeGoDependencies(
		ctx,
		t.TempDir(),
		"1000:1000",
		manifest,
	)

	if status.State != capsule.GoDependenciesUnknown {
		t.Fatalf(
			"state = %q, want unknown",
			status.State,
		)
	}

	if status.Reason == "" {
		t.Fatal("unknown status must include a reason")
	}
}
