package docker

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

func TestConfigureGoTargetPlatform(t *testing.T) {
	tests := []struct {
		name       string
		runtime    *RuntimeSpec
		targetOS   string
		targetArch string
		wantErr    bool
	}{
		{
			name:       "Linux AMD64",
			runtime:    &RuntimeSpec{Kind: "go", Version: "1.26.6"},
			targetOS:   "linux",
			targetArch: "amd64",
		},
		{
			name:       "macOS ARM64",
			runtime:    &RuntimeSpec{Kind: "go", Version: "1.26.6"},
			targetOS:   "darwin",
			targetArch: "arm64",
		},
		{
			name:       "unsupported Linux ARM64",
			runtime:    &RuntimeSpec{Kind: "go", Version: "1.26.6"},
			targetOS:   "linux",
			targetArch: "arm64",
			wantErr:    true,
		},
		{
			name:       "missing runtime",
			targetOS:   "darwin",
			targetArch: "arm64",
			wantErr:    true,
		},
		{
			name:       "unsupported Go version",
			runtime:    &RuntimeSpec{Kind: "go", Version: "1.25.0"},
			targetOS:   "darwin",
			targetArch: "arm64",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := &client.ContainerCreateOptions{
				Config: &container.Config{
					Env: []string{"CGO_ENABLED=0"},
				},
			}

			before := append([]string(nil), options.Config.Env...)

			err := configureGoTargetPlatform(
				options,
				tt.runtime,
				tt.targetOS,
				tt.targetArch,
			)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected unsupported target to be rejected")
				}

				if !reflect.DeepEqual(options.Config.Env, before) {
					t.Fatal("rejected target modified container environment")
				}

				return
			}

			if err != nil {
				t.Fatalf("configure target: %v", err)
			}

			env := strings.Join(options.Config.Env, "\n")

			if !strings.Contains(env+"\n", "GOOS="+tt.targetOS+"\n") {
				t.Fatalf("GOOS missing from environment: %v", options.Config.Env)
			}

			if !strings.Contains(env+"\n", "GOARCH="+tt.targetArch+"\n") {
				t.Fatalf("GOARCH missing from environment: %v", options.Config.Env)
			}
		})
	}
}

func TestSDKGoDarwinARM64BuildFailure(t *testing.T) {
	if os.Getenv("BUILDFOSSIL_DOCKER_TEST") != "1" {
		t.Skip("Docker integration test disabled")
	}

	workspace := t.TempDir()

	files := map[string]string{
		"go.mod":  "module example.com/cross-probe\n\ngo 1.26.0\n",
		"main.go": "package main\n\nfunc main() {\n    undefinedFunction()\n}\n",
	}

	for name, content := range files {
		if err := os.WriteFile(
			filepath.Join(workspace, name),
			[]byte(content),
			0600,
		); err != nil {
			t.Fatal(err)
		}
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		90*time.Second,
	)
	defer cancel()

	var stdout, stderr bytes.Buffer

	exitCode, err := RunWithSDKRuntimeTarget(
		ctx,
		workspace,
		[]string{"go", "build", "./..."},
		fmt.Sprintf("%d:%d", os.Geteuid(), os.Getegid()),
		&stdout,
		&stderr,
		&RuntimeSpec{
			Kind:    "go",
			Version: "1.26.6",
		},
		"darwin",
		"arm64",
	)
	if err != nil {
		t.Fatalf("cross-platform Go execution: %v", err)
	}

	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", exitCode)
	}

	const expected = "./main.go:4:5: undefined: undefinedFunction"

	if !strings.Contains(stderr.String(), expected) {
		t.Fatalf("unexpected stderr:\n%s", stderr.String())
	}

	t.Logf("Go darwin/arm64 stderr:\n%s", stderr.String())
}
