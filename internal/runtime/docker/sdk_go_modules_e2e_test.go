package docker

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestSDKOfflineGoModuleBuild(t *testing.T) {
	if os.Getenv("BUILDFOSSIL_DOCKER_TEST") != "1" {
		t.Skip("Docker integration test disabled")
	}

	fixture := makeOfflineGoFixture(t)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		150*time.Second,
	)
	defer cancel()

	command := []string{
		"sh",
		"-c",
		"mkdir -p /gomodcache/cache/download/example.com/buildfossil/fixture/@v && " +
			"cp /module-artifacts/v1.0.0.zip " +
			"/module-artifacts/v1.0.0.mod " +
			"/module-artifacts/v1.0.0.info " +
			"/gomodcache/cache/download/example.com/buildfossil/fixture/@v/ && " +
			"go build -mod=readonly -o /tmp/offline-build .",
	}

	var stdout, stderr bytes.Buffer

	exitCode, err := RunWithSDKRuntimeModules(
		ctx,
		fixture.workspace,
		command,
		fmt.Sprintf("%d:%d", os.Geteuid(), os.Getegid()),
		&stdout,
		&stderr,
		&RuntimeSpec{
			Kind:    "go",
			Version: "1.26.6",
		},
		fixture.artifactsDir,
	)

	if err != nil {
		t.Fatalf("Docker execution: %v\nstderr:\n%s", err, stderr.String())
	}

	if exitCode != 0 {
		t.Fatalf("offline build failed: exit=%d\nstderr:\n%s",
			exitCode, stderr.String())
	}

	if strings.Contains(stderr.String(), "module lookup disabled") {
		t.Fatalf("offline module lookup failed:\n%s", stderr.String())
	}
}
