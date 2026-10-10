package replay

import (
	"testing"

	"github.com/buildfossil/buildfossil/internal/capsule"
)

func TestPortableGoReplayCrossBuildV3(t *testing.T) {
	base := capsule.Manifest{
		SchemaVersion: capsule.SchemaVersionV3,
		Platform: capsule.Platform{
			OS:           "darwin",
			Architecture: "arm64",
		},
		Runtime: &capsule.Runtime{
			Kind:    "go",
			Version: capsule.SupportedGoVersion,
		},
		GoBuildEnv: &capsule.GoBuildEnvironment{
			GOOS:       "linux",
			GOARCH:     "amd64",
			CGOEnabled: "0",
		},
	}

	if err := validateReplayPlatform(base); err != nil {
		t.Fatalf("supported cross-build rejected: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*capsule.Manifest)
	}{
		{
			name: "CGO enabled",
			mutate: func(m *capsule.Manifest) {
				m.GoBuildEnv.CGOEnabled = "1"
			},
		},
		{
			name: "GOFLAGS present",
			mutate: func(m *capsule.Manifest) {
				m.GoBuildEnv.GOFLAGS = "-tags=custom"
			},
		},
		{
			name: "unsupported target",
			mutate: func(m *capsule.Manifest) {
				m.GoBuildEnv.GOARCH = "arm64"
			},
		},
		{
			name: "unsupported runtime",
			mutate: func(m *capsule.Manifest) {
				m.Runtime.Version = "1.25.0"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := base

			env := *base.GoBuildEnv
			runtime := *base.Runtime

			m.GoBuildEnv = &env
			m.Runtime = &runtime

			tt.mutate(&m)

			if err := validateReplayPlatform(m); err == nil {
				t.Fatal("unsupported portability configuration accepted")
			}
		})
	}
}
