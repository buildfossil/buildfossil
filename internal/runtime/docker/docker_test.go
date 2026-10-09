package docker

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDockerRunIntegration(t *testing.T) {
	if os.Getenv("BUILDFOSSIL_DOCKER_TEST") != "1" {
		t.Skip("set BUILDFOSSIL_DOCKER_TEST=1 to run Docker integration test")
	}

	workspace := t.TempDir()

	err := os.WriteFile(
		filepath.Join(workspace, "fixture.txt"),
		[]byte("BUILD_FOSSIL_TEST_FAILURE\n"),
		0644,
	)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)
	defer cancel()

	code, err := Run(
		ctx,
		workspace,
		[]string{
			"/bin/sh",
			"-c",
			"cat fixture.txt >&2; exit 17",
		},
		io.Discard,
		io.Discard,
	)

	if err != nil {
		t.Fatalf("Docker Run() error = %v", err)
	}

	if code != 17 {
		t.Fatalf("exit code = %d, want 17", code)
	}
}
