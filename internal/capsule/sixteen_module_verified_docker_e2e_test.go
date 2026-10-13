package capsule

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	dockerruntime "github.com/rewtools/rew/internal/runtime/docker"
)

func TestSixteenModuleVerifiedDockerReplayV3(t *testing.T) {
	if os.Getenv("REW_DOCKER_TEST") != "1" {
		t.Skip("Docker integration test disabled")
	}

	manifest, files, artifacts := makeEightModuleWorkspaceV3(t)

	for index := 9; index <= 16; index++ {
		modulePath := fmt.Sprintf("example.com/module%02d", index)

		mod, moduleArtifacts, moduleSum := makeIndexedGoModuleFixture(
			t, index, modulePath,
		)

		manifest.GoModules = append(manifest.GoModules, mod)

		for name, data := range moduleArtifacts {
			artifacts[name] = data
		}

		old := []byte(")\n")
		replacement := []byte(fmt.Sprintf(
			"\t%s v1.0.0\n)\n", modulePath,
		))

		updated := bytes.Replace(files[0].Data, old, replacement, 1)
		if bytes.Equal(updated, files[0].Data) {
			t.Fatal("failed to update go.mod")
		}

		files[0].Data = updated
		files[1].Data = append(files[1].Data, moduleSum...)
	}

	var archive bytes.Buffer

	if err := WriteWorkspaceV3(
		&archive, manifest, files, artifacts,
	); err != nil {
		t.Fatalf("write capsule: %v", err)
	}

	filename := filepath.Join(t.TempDir(), "sixteen.bfc")

	if err := os.WriteFile(filename, archive.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}

	verified, err := ReadVerifiedV3(filename)
	if err != nil {
		t.Fatalf("verify capsule: %v", err)
	}

	workspaceTAR, err := BuildWorkspaceSnapshotV3(verified)
	if err != nil {
		t.Fatal(err)
	}

	moduleTAR, err := BuildGoModuleSnapshotV3(verified)
	if err != nil {
		t.Fatal(err)
	}

	expected := make([]dockerruntime.ExpectedGoModule, 0, 16)

	for _, mod := range verified.Manifest.GoModules {
		expected = append(expected, dockerruntime.ExpectedGoModule{
			Path:    mod.Path,
			Version: mod.Version,
		})
	}

	if len(expected) != 16 {
		t.Fatalf("expected 16 modules, got %d", len(expected))
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
			"verified Docker replay failed: %v; stderr=%q",
			err, stderr.String(),
		)
	}

	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr=%q", code, stderr.String())
	}

	if !strings.Contains(stderr.String(), "undefinedSymbol") {
		t.Fatalf("expected compiler failure, got %q", stderr.String())
	}

	if stdout.Len() != 0 {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}

	t.Log("SIXTEEN-MODULE VERIFIED OFFLINE DOCKER REPLAY: PASS")

	wrongGraph := append(
		[]dockerruntime.ExpectedGoModule(nil),
		expected...,
	)
	wrongGraph[15].Version = "v9.0.0"

	stdout.Reset()
	stderr.Reset()

	const marker = "BUILDFOSSIL_UNEXPECTED_COMMAND_EXECUTION"

	_, err = dockerruntime.RunWithSDKRuntimeSnapshotExecVerified(
		ctx,
		workspaceTAR,
		moduleTAR,
		[]string{
			"/bin/sh", "-c",
			"printf 'BUILDFOSSIL_UNEXPECTED_COMMAND_EXECUTION\\n' >&2",
		},
		wrongGraph,
		"10001:10001",
		&stdout,
		&stderr,
		&dockerruntime.RuntimeSpec{
			Kind:    "go",
			Version: SupportedGoVersion,
		},
	)

	if err == nil {
		t.Fatal("mismatched sixteen-module graph accepted")
	}

	if strings.Contains(stderr.String(), marker) ||
		strings.Contains(stdout.String(), marker) {
		t.Fatal("command executed despite module graph mismatch")
	}

	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf(
			"unexpected command output: stdout=%q stderr=%q",
			stdout.String(),
			stderr.String(),
		)
	}

	t.Log("SIXTEEN-MODULE GRAPH MISMATCH REJECTED: PASS")

}
