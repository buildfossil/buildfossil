package replay

import (
	"strings"
	"testing"

	"github.com/buildfossil/buildfossil/internal/capsule"
)

func TestValidateReplayPlatform(t *testing.T) {
	tests := []struct {
		name        string
		platform    capsule.Platform
		runtime     *capsule.Runtime
		goBuildEnv  *capsule.GoBuildEnvironment
		wantAllowed bool
	}{
		{
			name: "legacy Linux AMD64",
			platform: capsule.Platform{
				OS: "linux", Architecture: "amd64",
			},
			wantAllowed: true,
		},
		{
			name: "Go Linux AMD64",
			platform: capsule.Platform{
				OS: "linux", Architecture: "amd64",
			},
			runtime: &capsule.Runtime{
				Kind: "go", Version: capsule.SupportedGoVersion,
			},
			wantAllowed: true,
		},
		{
			name: "Go macOS ARM64",
			platform: capsule.Platform{
				OS: "darwin", Architecture: "arm64",
			},
			runtime: &capsule.Runtime{
				Kind: "go", Version: capsule.SupportedGoVersion,
			},
			goBuildEnv: &capsule.GoBuildEnvironment{
				GOOS:       "darwin",
				GOARCH:     "arm64",
				CGOEnabled: "0",
				GOFLAGS:    "",
			},
			wantAllowed: true,
		},
		{
			name: "legacy macOS ARM64",
			platform: capsule.Platform{
				OS: "darwin", Architecture: "arm64",
			},
			wantAllowed: false,
		},
		{
			name: "Go macOS ARM64 with CGO",
			platform: capsule.Platform{
				OS: "darwin", Architecture: "arm64",
			},
			runtime: &capsule.Runtime{
				Kind: "go", Version: capsule.SupportedGoVersion,
			},
			goBuildEnv: &capsule.GoBuildEnvironment{
				GOOS:       "darwin",
				GOARCH:     "arm64",
				CGOEnabled: "1",
			},
			wantAllowed: false,
		},
		{
			name: "Linux ARM64",
			platform: capsule.Platform{
				OS: "linux", Architecture: "arm64",
			},
			wantAllowed: false,
		},
		{
			name: "Windows AMD64",
			platform: capsule.Platform{
				OS: "windows", Architecture: "amd64",
			},
			wantAllowed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest := capsule.Manifest{
				Platform:   tt.platform,
				Runtime:    tt.runtime,
				GoBuildEnv: tt.goBuildEnv,
			}

			err := validateReplayPlatform(manifest)

			if tt.wantAllowed {
				if err != nil {
					t.Fatalf("expected platform to be allowed: %v", err)
				}
				return
			}

			if err == nil {
				t.Fatal("expected unsupported platform to be rejected")
			}

			if !strings.Contains(err.Error(), "replay:") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
