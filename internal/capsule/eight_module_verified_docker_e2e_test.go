package capsule

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	dockerruntime "github.com/buildfossil/buildfossil/internal/runtime/docker"
)

func TestEightModuleVerifiedDockerReplayV3(t *testing.T) {
	if os.Getenv("BUILDFOSSIL_DOCKER_TEST") != "1" {
		t.Skip("Docker integration test disabled")
	}

	manifest, files, artifacts := makeEightModuleWorkspaceV3(t)

	var archive bytes.Buffer
	if err := WriteWorkspaceV3(
		&archive, manifest, files, artifacts,
	); err != nil {
		t.Fatalf("write eight-module capsule: %v", err)
	}

	filename := filepath.Join(t.TempDir(), "eight.bfc")
	if err := os.WriteFile(filename, archive.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}

	verified, err := ReadVerifiedV3(filename)
	if err != nil {
		t.Fatalf("verify eight-module capsule: %v", err)
	}

	workspaceTAR, err := BuildWorkspaceSnapshotV3(verified)
	if err != nil {
		t.Fatalf("workspace snapshot: %v", err)
	}

	moduleTAR, err := BuildGoModuleSnapshotV3(verified)
	if err != nil {
		t.Fatalf("module snapshot: %v", err)
	}

	expected := make(
		[]dockerruntime.ExpectedGoModule,
		0,
		len(verified.Manifest.GoModules),
	)
	for _, mod := range verified.Manifest.GoModules {
		expected = append(expected, dockerruntime.ExpectedGoModule{
			Path:    mod.Path,
			Version: mod.Version,
		})
	}

	if len(expected) != 8 {
		t.Fatalf("module count = %d; want 8", len(expected))
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		180*time.Second,
	)
	defer cancel()

	var stdout, stderr bytes.Buffer

	code, err := dockerruntime.RunWithSDKRuntimeSnapshotExecVerified(
		ctx,
		workspaceTAR,
		moduleTAR,
		[]string{"go", "build", "./..."},
		expected,
		"10001:10001",
		&stdout,
		&stderr,
		&dockerruntime.RuntimeSpec{
			Kind:    "go",
			Version: SupportedGoVersion,
		},
	)
	if err != nil {
		t.Fatalf(
			"verified Docker execution failed: %v; stderr=%q",
			err, stderr.String(),
		)
	}

	if code != 1 {
		t.Fatalf(
			"exit code = %d; want 1; stderr=%q",
			code, stderr.String(),
		)
	}

	if !strings.Contains(stderr.String(), "undefinedSymbol") {
		t.Fatalf("expected compiler failure, got %q", stderr.String())
	}

	if stdout.Len() != 0 {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}

	t.Log("EIGHT-MODULE VERIFIED OFFLINE DOCKER REPLAY: PASS")
}
