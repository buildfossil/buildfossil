package capsule

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	dockerruntime "github.com/rewtools/rew/internal/runtime/docker"
)

func TestMultiModuleV3DockerOfflineReplay(t *testing.T) {
	if os.Getenv("REW_DOCKER_TEST") != "1" {
		t.Skip("Docker integration test disabled")
	}

	verified := makeVerifiedMultiModuleSnapshotV3(t)

	// Ensure the build really imports and compiles both modules.
	source := []byte(`package main

import (
	a "example.com/a"
	b "example.com/b"
)

func main() {
	if a.Value()+b.Value() != 84 {
		panic("unexpected module values")
	}
}
`)

	found := false
	for i := range verified.Files {
		if verified.Files[i].Path != "main.go" {
			continue
		}

		verified.Files[i].Data = source
		found = true

		for j := range verified.Manifest.Workspace.Files {
			meta := &verified.Manifest.Workspace.Files[j]
			if meta.Path == "main.go" {
				meta.Size = int64(len(source))
				meta.SHA256 = SHA256(source)
				break
			}
		}
		break
	}

	if !found {
		t.Fatal("fixture missing main.go")
	}

	workspaceTAR, err := BuildWorkspaceSnapshotV3(verified)
	if err != nil {
		t.Fatalf("workspace snapshot: %v", err)
	}

	moduleTAR, err := BuildGoModuleSnapshotV3(verified)
	if err != nil {
		t.Fatalf("module snapshot: %v", err)
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		180*time.Second,
	)
	defer cancel()

	var stdout, stderr bytes.Buffer

	code, err := dockerruntime.RunWithSDKRuntimeSnapshotExec(
		ctx,
		workspaceTAR,
		moduleTAR,
		[]string{"go", "list", "-m", "-json", "all"},
		"10001:10001",
		&stdout,
		&stderr,
		&dockerruntime.RuntimeSpec{
			Kind: "go", Version: "1.26.6",
		},
	)
	if err != nil {
		t.Fatalf("Docker execution: %v; stderr=%s", err, stderr.String())
	}
	if code != 0 {
		t.Fatalf("offline graph exit=%d; stderr=%s", code, stderr.String())
	}

	resolved, err := ParseGoModuleGraphV3(stdout.Bytes())
	if err != nil {
		t.Fatalf("parse offline module graph: %v", err)
	}

	if err := CompareGoModuleGraphV3(
		verified.Manifest.GoModules,
		resolved,
	); err != nil {
		t.Fatalf("offline graph mismatch: %v", err)
	}

	t.Log("EXACT OFFLINE MODULE GRAPH: PASS")

	stdout.Reset()
	stderr.Reset()

	code, err = dockerruntime.RunWithSDKRuntimeSnapshotExec(
		ctx,
		workspaceTAR,
		moduleTAR,
		[]string{"go", "build", "-mod=readonly", "-o", "/tmp/buildfossil-test", "./..."},
		"10001:10001",
		&stdout,
		&stderr,
		&dockerruntime.RuntimeSpec{
			Kind: "go", Version: "1.26.6",
		},
	)
	if err != nil {
		t.Fatalf("offline build infrastructure error: %v; stderr=%s", err, stderr.String())
	}
	if code != 0 {
		t.Fatalf("offline build exit=%d; stderr=%s", code, stderr.String())
	}

	t.Log("TWO-MODULE OFFLINE GO BUILD: PASS")
}
