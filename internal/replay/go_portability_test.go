package replay

import (
	"strings"
	"testing"

	"github.com/buildfossil/buildfossil/internal/capsule"
)

func TestValidatePortableGoReplay(t *testing.T) {
	newManifest := func() capsule.Manifest {
		return capsule.Manifest{
			SchemaVersion: capsule.SchemaVersionV2,
			Platform: capsule.Platform{
				OS:           "darwin",
				Architecture: "arm64",
			},
			Runtime: &capsule.Runtime{
				Kind:    "go",
				Version: capsule.SupportedGoVersion,
			},
			GoBuildEnv: &capsule.GoBuildEnvironment{
				GOOS:       "darwin",
				GOARCH:     "arm64",
				CGOEnabled: "0",
				GOFLAGS:    "",
			},
		}
	}

	tests := []struct {
		name    string
		modify  func(*capsule.Manifest)
		wantErr string
	}{
		{
			name: "supported macOS ARM64",
		},
		{
			name: "CGO enabled",
			modify: func(m *capsule.Manifest) {
				m.GoBuildEnv.CGOEnabled = "1"
			},
			wantErr: "CGO_ENABLED=0",
		},
		{
			name: "custom GOFLAGS",
			modify: func(m *capsule.Manifest) {
				m.GoBuildEnv.GOFLAGS = "-tags=custom"
			},
			wantErr: "empty GOFLAGS",
		},
		{
			name: "missing environment",
			modify: func(m *capsule.Manifest) {
				m.GoBuildEnv = nil
			},
			wantErr: "requires recorded build environment",
		},
		{
			name: "platform mismatch",
			modify: func(m *capsule.Manifest) {
				m.GoBuildEnv.GOOS = "linux"
			},
			wantErr: "does not match capture platform",
		},
		{
			name: "missing runtime",
			modify: func(m *capsule.Manifest) {
				m.Runtime = nil
			},
			wantErr: "requires supported Go runtime",
		},
		{
			name: "unsupported Go version",
			modify: func(m *capsule.Manifest) {
				m.Runtime.Version = "1.25.0"
			},
			wantErr: "requires supported Go runtime",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest := newManifest()

			if tt.modify != nil {
				tt.modify(&manifest)
			}

			err := validatePortableGoReplay(manifest)

			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("expected portable replay: %v", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("expected error containing %q", tt.wantErr)
			}

			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf(
					"error = %v, want substring %q",
					err,
					tt.wantErr,
				)
			}
		})
	}
}
