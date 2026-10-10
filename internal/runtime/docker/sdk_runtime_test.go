package docker

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSelectReplayImage(t *testing.T) {
	tests := []struct {
		name    string
		runtime *RuntimeSpec
		want    string
		wantErr bool
	}{
		{
			name: "legacy runtime",
			want: replayImage,
		},
		{
			name: "Go 1.26.6",
			runtime: &RuntimeSpec{
				Kind:    "go",
				Version: "1.26.6",
			},
			want: goReplayImage,
		},
		{
			name: "unsupported Go",
			runtime: &RuntimeSpec{
				Kind:    "go",
				Version: "1.25.0",
			},
			wantErr: true,
		},
		{
			name: "unknown runtime",
			runtime: &RuntimeSpec{
				Kind:    "python",
				Version: "3.14",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := selectReplayImage(tt.runtime)

			if (err != nil) != tt.wantErr {
				t.Fatalf("unexpected error: %v", err)
			}

			if !tt.wantErr && got != tt.want {
				t.Fatalf("image = %q; want %q", got, tt.want)
			}
		})
	}
}
func TestSDKGoRuntimeBuildFailure(t *testing.T) {
	if os.Getenv("BUILDFOSSIL_DOCKER_TEST") != "1" {
		t.Skip("Docker integration test disabled")
	}

	workspace := t.TempDir()

	if err := os.WriteFile(
		filepath.Join(workspace, "go.mod"),
		[]byte("module example.com/buildfossil-probe\n\ngo 1.26.0\n"),
		0600,
	); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(
		filepath.Join(workspace, "main.go"),
		[]byte("package main\n\nfunc main() {\n    undefinedFunction()\n}\n"),
		0600,
	); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		90*time.Second,
	)
	defer cancel()

	var stdout, stderr bytes.Buffer

	exitCode, err := RunWithSDKRuntime(
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
	)
	if err != nil {
		t.Fatalf("Go runtime execution: %v\nstderr: %s", err, stderr.String())
	}

	if exitCode == 0 {
		t.Fatal("expected Go compilation failure")
	}

	if !strings.Contains(stderr.String(), "undefined: undefinedFunction") {
		t.Fatalf(
			"unexpected compilation error:\n%s",
			stderr.String(),
		)
	}

	t.Logf("Go build exit code: %d", exitCode)
	t.Logf("Go compiler stderr:\n%s", stderr.String())
}
