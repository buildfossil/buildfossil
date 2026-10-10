package replay

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/buildfossil/buildfossil/internal/capsule"
)

func TestGoDependencyProbeDockerE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Docker E2E skipped in short mode")
	}

	if os.Geteuid() == 0 {
		t.Skip("Docker replay refuses root execution")
	}

	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Docker CLI unavailable")
	}

	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skip("Docker daemon unavailable")
	}

	tests := []struct {
		name      string
		goMod     string
		mainGo    string
		wantState capsule.GoDependencyState
	}{
		{
			name: "no external dependencies",
			goMod: `module example.com/probe

go 1.26.6
`,
			mainGo: `package main

func main() {
	undefinedFunction()
}
`,
			wantState: capsule.GoDependenciesReady,
		},
		{
			name: "unused external dependency",
			goMod: `module example.com/probe

go 1.26.6

require github.com/google/uuid v1.6.0
`,
			mainGo: `package main

func main() {
	undefinedFunction()
}
`,
			wantState: capsule.GoDependenciesReady,
		},
		{
			name: "missing external dependency",
			goMod: `module example.com/probe

go 1.26.6

require github.com/google/uuid v1.6.0
`,
			mainGo: `package main

import "github.com/google/uuid"

func main() {
	_ = uuid.NewString()
}
`,
			wantState: capsule.GoDependenciesUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workspace := t.TempDir()

			for name, content := range map[string]string{
				"go.mod":  tt.goMod,
				"main.go": tt.mainGo,
			} {
				if err := os.WriteFile(
					filepath.Join(workspace, name),
					[]byte(content),
					0644,
				); err != nil {
					t.Fatal(err)
				}
			}

			m := capsule.Manifest{
				SchemaVersion: capsule.SchemaVersionV2,
				Runtime: &capsule.Runtime{
					Kind:    "go",
					Version: capsule.SupportedGoVersion,
				},
				Platform: capsule.Platform{
					OS:           runtime.GOOS,
					Architecture: runtime.GOARCH,
				},
				GoBuildEnv: &capsule.GoBuildEnvironment{
					GOOS:       runtime.GOOS,
					GOARCH:     runtime.GOARCH,
					CGOEnabled: "0",
				},
			}

			before := make(map[string][32]byte)

			for _, name := range []string{"go.mod", "main.go"} {
				data, err := os.ReadFile(filepath.Join(workspace, name))
				if err != nil {
					t.Fatal(err)
				}

				before[name] = sha256.Sum256(data)
			}

			status := probeGoDependencies(
				context.Background(),
				workspace,
				fmt.Sprintf("%d:%d", os.Geteuid(), os.Getegid()),
				m,
			)

			for name, expected := range before {
				data, err := os.ReadFile(filepath.Join(workspace, name))
				if err != nil {
					t.Fatalf("read %s after probe: %v", name, err)
				}

				if got := sha256.Sum256(data); got != expected {
					t.Fatalf("probe modified workspace file %s", name)
				}
			}

			if status.State != tt.wantState {
				t.Fatalf(
					"state = %q, want %q, reason = %q",
					status.State,
					tt.wantState,
					status.Reason,
				)
			}

			if status.State == capsule.GoDependenciesReady &&
				strings.TrimSpace(status.Reason) != "" {
				t.Fatalf("ready state has reason: %q", status.Reason)
			}
		})
	}
}
