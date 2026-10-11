package capsule

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	dockerruntime "github.com/buildfossil/buildfossil/internal/runtime/docker"
)

func TestLargeWorkspaceV3DockerE2E(t *testing.T) {
	if os.Getenv("BUILDFOSSIL_DOCKER_TEST") != "1" {
		t.Skip("Docker integration test disabled")
	}
	if os.Geteuid() == 0 {
		t.Skip("Docker runtime refuses root")
	}

	manifest, files := largeWorkspaceV3Fixture()

	const badPath = "src/file-000.go"
	found := false

	for i := range files {
		if files[i].Path == badPath {
			files[i].Data = []byte(
				"package fixture\n\nvar Value0 = missingSymbol\n",
			)
			found = true
			break
		}
	}

	if !found {
		t.Fatal("failure source not found")
	}

	if len(files) != MaxWorkspaceV3Files {
		t.Fatalf("expected %d files, got %d",
			MaxWorkspaceV3Files, len(files))
	}

	var archive bytes.Buffer
	if err := WriteWorkspaceV3(&archive, manifest, files, nil); err != nil {
		t.Fatalf("write capsule: %v", err)
	}

	capsulePath := t.TempDir() + "/large-docker.bfc"
	if err := os.WriteFile(capsulePath, archive.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}

	verified, err := ReadVerifiedV3(capsulePath)
	if err != nil {
		t.Fatalf("verify capsule: %v", err)
	}

	workspaceTAR, err := BuildWorkspaceSnapshotV3(verified)
	if err != nil {
		t.Fatalf("build workspace snapshot: %v", err)
	}

	moduleTAR, err := BuildGoModuleSnapshotV3(verified)
	if err != nil {
		t.Fatalf("build module snapshot: %v", err)
	}

	ctx, cancel := context.WithTimeout(
		context.Background(), 180*time.Second,
	)
	defer cancel()

	var stdout, stderr bytes.Buffer

	code, err := dockerruntime.RunWithSDKRuntimeSnapshotExecVerified(
		ctx,
		workspaceTAR,
		moduleTAR,
		[]string{"go", "build", "./..."},
		[]dockerruntime.ExpectedGoModule{},
		"10001:10001",
		&stdout,
		&stderr,
		&dockerruntime.RuntimeSpec{
			Kind: "go", Version: SupportedGoVersion,
		},
	)

	if err != nil {
		t.Fatalf("Docker infrastructure error: %v; stderr=%s",
			err, stderr.String())
	}

	if code != 1 {
		t.Fatalf("Docker build exit=%d, want 1; stderr=%s",
			code, stderr.String())
	}

	if stdout.Len() != 0 {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}

	if !strings.Contains(stderr.String(), "undefined: missingSymbol") {
		t.Fatalf("original compiler failure not reproduced: %q",
			stderr.String())
	}

	if !strings.Contains(
		stderr.String(), "src/file-000.go",
	) {
		t.Fatalf("compiler error references wrong source: %q",
			stderr.String())
	}

	t.Logf("256-FILE DOCKER BUILD: exit=%d", code)
	t.Logf("Compiler stderr:\n%s", stderr.String())
}
