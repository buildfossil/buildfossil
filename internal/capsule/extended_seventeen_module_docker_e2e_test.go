package capsule

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	dockerruntime "github.com/rewtools/rew/internal/runtime/docker"
)

func TestExtendedSeventeenModuleDockerReplayV3(t *testing.T) {
	if os.Getenv("REW_DOCKER_TEST") != "1" {
		t.Skip("Docker integration test disabled")
	}

	manifest, files, artifacts :=
		makeExtendedSeventeenModuleWorkspaceV3(t)

	var archive bytes.Buffer
	if err := WriteWorkspaceV3(
		&archive, manifest, files, artifacts,
	); err != nil {
		t.Fatalf("write extended capsule: %v", err)
	}

	filename := filepath.Join(t.TempDir(), "extended-17.bfc")
	if err := os.WriteFile(filename, archive.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}

	verified, err := ReadVerifiedV3(filename)
	if err != nil {
		t.Fatalf("verify capsule: %v", err)
	}

	workspaceTAR, err := BuildWorkspaceSnapshotV3(verified)
	if err != nil {
		t.Fatalf("workspace snapshot: %v", err)
	}

	moduleTAR, err := BuildGoModuleSnapshotV3(verified)
	if err != nil {
		t.Fatalf("module snapshot: %v", err)
	}

	selected := EffectiveGoSelectedModulesV3(verified.Manifest)
	if len(selected) != 17 {
		t.Fatalf("selected modules=%d, want 17", len(selected))
	}

	expected := make([]dockerruntime.ExpectedGoModule, 0, len(selected))
	for _, mod := range selected {
		expected = append(expected, dockerruntime.ExpectedGoModule{
			Path: mod.Path, Version: mod.Version,
		})
	}

	materialized := make(
		[]dockerruntime.ExpectedGoModule,
		0,
		len(verified.Manifest.GoModules),
	)
	for _, mod := range verified.Manifest.GoModules {
		materialized = append(materialized, dockerruntime.ExpectedGoModule{
			Path: mod.Path, Version: mod.Version,
		})
	}

	if len(materialized) != 5 {
		t.Fatalf("materialized modules=%d, want 5", len(materialized))
	}

	ctx, cancel := context.WithTimeout(
		context.Background(), 180*time.Second,
	)
	defer cancel()

	var stdout, stderr bytes.Buffer
	code, err := dockerruntime.RunWithSDKRuntimeSnapshotExecSelectiveV3(
		ctx,
		workspaceTAR,
		moduleTAR,
		[]string{"go", "build", "./..."},
		expected,
		materialized,
		1<<30,
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
			"selective Docker execution: %v; stderr=%q",
			err, stderr.String(),
		)
	}

	if code != 1 {
		t.Fatalf(
			"exit code=%d, want 1; stderr=%q",
			code, stderr.String(),
		)
	}

	if !strings.Contains(stderr.String(), "undefinedSymbol") {
		t.Fatalf("unexpected compiler stderr: %q", stderr.String())
	}

	if stdout.Len() != 0 {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}

	t.Log(
		"EXTENDED 17 SELECTED / 5 ZIP / 12 METADATA-ONLY OFFLINE DOCKER REPLAY: PASS",
	)
}
