package replay

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buildfossil/buildfossil/internal/capsule"
)

func TestReplayV2RejectsCommandWithoutOptIn(t *testing.T) {
	manifest := capsule.Manifest{
		SchemaVersion: capsule.SchemaVersionV2,
		Execution: capsule.Execution{
			Argv:       []string{"/bin/sh", "-c", "echo UNEXPECTED_EXECUTION >&2; exit 23"},
			WorkingDir: ".",
			ExitCode:   23,
			Stderr:     "UNEXPECTED_EXECUTION\n",
		},
		Platform: capsule.Platform{
			OS:           "linux",
			Architecture: "amd64",
		},
	}

	files := []capsule.WorkspaceFile{
		{
			Path: "go.mod",
			Data: []byte("module example.com/demo\n"),
			Mode: 0600,
		},
	}

	var archive bytes.Buffer

	if err := capsule.WriteWorkspaceV2(&archive, manifest, files); err != nil {
		t.Fatalf("write v2 capsule: %v", err)
	}

	output := filepath.Join(t.TempDir(), "failure.bfc")

	if err := os.WriteFile(output, archive.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := RunV2WithOptions(
		context.Background(),
		output,
		Options{},
	)

	if err == nil {
		t.Fatal("expected v2 replay to require explicit opt-in")
	}

	if !strings.Contains(err.Error(), "requires explicit opt-in") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReplayV2DockerIntegration(t *testing.T) {
	if os.Getenv("BUILDFOSSIL_DOCKER_TEST") != "1" {
		t.Skip("set BUILDFOSSIL_DOCKER_TEST=1 to run Docker tests")
	}

	manifest := capsule.Manifest{
		SchemaVersion: capsule.SchemaVersionV2,
		Execution: capsule.Execution{
			Argv: []string{
				"/bin/sh",
				"-c",
				"cat go.mod >&2; cat src/main.go >&2; exit 23",
			},
			WorkingDir: ".",
			ExitCode:   23,
			Stderr:     "module example.com/linux-demo\npackage main\n",
		},
		Platform: capsule.Platform{
			OS:           "linux",
			Architecture: "amd64",
		},
	}

	files := []capsule.WorkspaceFile{
		{
			Path: "go.mod",
			Data: []byte("module example.com/linux-demo\n"),
			Mode: 0600,
		},
		{
			Path: "src/main.go",
			Data: []byte("package main\n"),
			Mode: 0600,
		},
	}

	var archive bytes.Buffer

	if err := capsule.WriteWorkspaceV2(&archive, manifest, files); err != nil {
		t.Fatalf("write v2 capsule: %v", err)
	}

	output := filepath.Join(t.TempDir(), "failure.bfc")

	if err := os.WriteFile(output, archive.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)
	defer cancel()

	result, err := RunV2WithOptions(
		ctx,
		output,
		Options{AllowArbitraryCommand: true},
	)
	if err != nil {
		t.Fatalf("replay v2: %v", err)
	}

	if result.OriginalExitCode != 23 {
		t.Errorf("original exit = %d, want 23", result.OriginalExitCode)
	}

	if result.ReplayExitCode != 23 {
		t.Errorf("replay exit = %d, want 23", result.ReplayExitCode)
	}

	if result.Outcome != OutcomeReproduced {
		t.Fatalf(
			"outcome = %q, want %q",
			result.Outcome,
			OutcomeReproduced,
		)
	}
}

func TestReplayV2GoBuildIntegration(t *testing.T) {
	if os.Getenv("BUILDFOSSIL_DOCKER_TEST") != "1" {
		t.Skip("set BUILDFOSSIL_DOCKER_TEST=1 to run Docker tests")
	}

	const compilerStderr = "# example.com/buildfossil-probe\n" +
		"./main.go:4:5: undefined: undefinedFunction\n"

	manifest := capsule.Manifest{
		SchemaVersion: capsule.SchemaVersionV2,
		Runtime: &capsule.Runtime{
			Kind:    "go",
			Version: capsule.SupportedGoVersion,
		},
		GoBuildEnv: &capsule.GoBuildEnvironment{
			GOOS:       "linux",
			GOARCH:     "amd64",
			CGOEnabled: "0",
		},
		Execution: capsule.Execution{
			Argv:       []string{"go", "build", "./..."},
			WorkingDir: ".",
			ExitCode:   1,
			Stderr:     compilerStderr,
		},
		Platform: capsule.Platform{
			OS:           "linux",
			Architecture: "amd64",
		},
	}

	files := []capsule.WorkspaceFile{
		{
			Path: "go.mod",
			Data: []byte(
				"module example.com/buildfossil-probe\n\ngo 1.26.0\n",
			),
			Mode: 0600,
		},
		{
			Path: "main.go",
			Data: []byte(
				"package main\n\nfunc main() {\n    undefinedFunction()\n}\n",
			),
			Mode: 0600,
		},
	}

	var archive bytes.Buffer

	if err := capsule.WriteWorkspaceV2(
		&archive,
		manifest,
		files,
	); err != nil {
		t.Fatalf("write Go capsule: %v", err)
	}

	capsulePath := filepath.Join(t.TempDir(), "go-failure.bfc")

	if err := os.WriteFile(
		capsulePath,
		archive.Bytes(),
		0600,
	); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		90*time.Second,
	)
	defer cancel()

	result, err := RunV2WithOptions(
		ctx,
		capsulePath,
		Options{AllowArbitraryCommand: true},
	)
	if err != nil {
		t.Fatalf("Go replay: %v", err)
	}

	if result.OriginalExitCode != 1 {
		t.Fatalf(
			"original exit = %d, want 1",
			result.OriginalExitCode,
		)
	}

	if result.ReplayExitCode != 1 {
		t.Fatalf(
			"replay exit = %d, want 1",
			result.ReplayExitCode,
		)
	}

	if result.Outcome != OutcomeReproduced {
		t.Fatalf(
			"outcome = %q, want %q",
			result.Outcome,
			OutcomeReproduced,
		)
	}

	if result.GoDependencies != nil {
		t.Fatal("dependency diagnostics ran without opt-in")
	}

	diagnosed, err := RunV2WithOptions(
		ctx,
		capsulePath,
		Options{
			AllowArbitraryCommand: true,
			DiagnoseDependencies:  true,
		},
	)
	if err != nil {
		t.Fatalf("Go replay with diagnostics: %v", err)
	}

	if diagnosed.Outcome != result.Outcome {
		t.Fatalf(
			"diagnostics changed outcome: %q -> %q",
			result.Outcome,
			diagnosed.Outcome,
		)
	}

	if diagnosed.OriginalExitCode != result.OriginalExitCode ||
		diagnosed.ReplayExitCode != result.ReplayExitCode {
		t.Fatalf("diagnostics changed replay exit codes")
	}

	if diagnosed.GoDependencies == nil {
		t.Fatal("missing Go dependency diagnostics")
	}

	if diagnosed.GoDependencies.State != capsule.GoDependenciesReady {
		t.Fatalf(
			"dependency state = %q, want ready; reason = %q",
			diagnosed.GoDependencies.State,
			diagnosed.GoDependencies.Reason,
		)
	}

	t.Logf("Go Replay outcome: %s", result.Outcome)
}

func TestReplayV2MissingDependencyDiagnostics(t *testing.T) {
	if os.Getenv("BUILDFOSSIL_DOCKER_TEST") != "1" {
		t.Skip("set BUILDFOSSIL_DOCKER_TEST=1 to run Docker tests")
	}

	manifest := capsule.Manifest{
		SchemaVersion: capsule.SchemaVersionV2,
		Runtime: &capsule.Runtime{
			Kind:    "go",
			Version: capsule.SupportedGoVersion,
		},
		GoBuildEnv: &capsule.GoBuildEnvironment{
			GOOS:       "linux",
			GOARCH:     "amd64",
			CGOEnabled: "0",
		},
		Execution: capsule.Execution{
			Argv:       []string{"go", "build", "./..."},
			WorkingDir: ".",
			ExitCode:   1,
			Stderr:     "# example.com/missing-dependency\n./main.go:4:2: undefined: undefinedFunction\n",
		},
		Platform: capsule.Platform{
			OS:           "linux",
			Architecture: "amd64",
		},
	}

	files := []capsule.WorkspaceFile{
		{
			Path: "go.mod",
			Data: []byte(`module example.com/missing-dependency

go 1.26.0

require github.com/google/uuid v1.6.0
`),
			Mode: 0600,
		},
		{
			Path: "main.go",
			Data: []byte(`package main

import "github.com/google/uuid"

func main() {
	_ = uuid.NewString()
	undefinedFunction()
}
`),
			Mode: 0600,
		},
	}

	var archive bytes.Buffer

	if err := capsule.WriteWorkspaceV2(&archive, manifest, files); err != nil {
		t.Fatalf("write capsule: %v", err)
	}

	capsulePath := filepath.Join(t.TempDir(), "missing-dependency.bfc")

	if err := os.WriteFile(capsulePath, archive.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		120*time.Second,
	)
	defer cancel()

	result, err := RunV2WithOptions(
		ctx,
		capsulePath,
		Options{
			AllowArbitraryCommand: true,
			DiagnoseDependencies:  true,
		},
	)
	if err != nil {
		t.Fatalf("replay with diagnostics: %v", err)
	}

	if result.Outcome != OutcomeDifferent {
		t.Fatalf(
			"outcome = %q, want %q",
			result.Outcome,
			OutcomeDifferent,
		)
	}

	if result.GoDependencies == nil {
		t.Fatal("missing dependency diagnostics")
	}

	if result.GoDependencies.State != capsule.GoDependenciesUnknown {
		t.Fatalf(
			"dependency state = %q, want unknown; reason = %q",
			result.GoDependencies.State,
			result.GoDependencies.Reason,
		)
	}
}
