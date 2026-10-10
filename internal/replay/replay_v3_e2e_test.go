package replay

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/buildfossil/buildfossil/internal/capsule"
	"github.com/buildfossil/buildfossil/internal/runtime/docker"
	"golang.org/x/mod/sumdb/dirhash"
)

func TestReplayV3OfflineGoFailure(t *testing.T) {
	if os.Getenv("BUILDFOSSIL_DOCKER_TEST") != "1" {
		t.Skip("Docker E2E disabled")
	}
	if os.Geteuid() == 0 {
		t.Skip("Docker runner refuses root")
	}

	const modPath = "example.com/buildfossil/fixture"
	const modVersion = "v1.0.0"

	ctx, cancel := context.WithTimeout(
		context.Background(), 240*time.Second,
	)
	defer cancel()

	moduleGoMod := []byte(
		"module " + modPath + "\n\ngo 1.26.0\n",
	)
	moduleSource := []byte(
		"package fixture\n\nfunc Value() string { return \"offline-ok\" }\n",
	)

	moduleZip := filepath.Join(t.TempDir(), "fixture.zip")
	f, err := os.Create(moduleZip)
	if err != nil {
		t.Fatal(err)
	}

	zw := zip.NewWriter(f)
	for _, entry := range []struct {
		name string
		data []byte
	}{
		{modPath + "@" + modVersion + "/go.mod", moduleGoMod},
		{modPath + "@" + modVersion + "/fixture.go", moduleSource},
	} {
		w, err := zw.Create(entry.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(entry.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	zipData, err := os.ReadFile(moduleZip)
	if err != nil {
		t.Fatal(err)
	}

	moduleHash, err := dirhash.HashZip(moduleZip, dirhash.Hash1)
	if err != nil {
		t.Fatal(err)
	}

	modHash, err := dirhash.Hash1(
		[]string{"go.mod"},
		func(name string) (io.ReadCloser, error) {
			if name != "go.mod" {
				return nil, fmt.Errorf("unexpected file %s", name)
			}
			return io.NopCloser(bytes.NewReader(moduleGoMod)), nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	artifacts := map[string][]byte{
		"module-0001.zip":  zipData,
		"module-0001.mod":  moduleGoMod,
		"module-0001.info": []byte(`{"Version":"v1.0.0"}`),
	}

	moduleArtifacts := make([]capsule.GoModuleArtifact, 0, 3)
	for _, ext := range []string{".zip", ".mod", ".info"} {
		name := "module-0001" + ext
		data := artifacts[name]
		moduleArtifacts = append(moduleArtifacts, capsule.GoModuleArtifact{
			Path:   name,
			Size:   int64(len(data)),
			SHA256: capsule.SHA256(data),
		})
	}

	goMod := []byte(
		"module example.com/replay-v3-test\n\ngo 1.26.0\n" +
			"require " + modPath + " " + modVersion + "\n",
	)
	goSum := []byte(fmt.Sprintf(
		"%s %s %s\n%s %s/go.mod %s\n",
		modPath, modVersion, moduleHash,
		modPath, modVersion, modHash,
	))
	mainGo := []byte(`package main

import "example.com/buildfossil/fixture"

func main() {
	_ = fixture.Value()
	missingSymbol()
}
`)

	files := []capsule.WorkspaceFile{
		{Path: "go.mod", Data: goMod, Mode: 0600},
		{Path: "go.sum", Data: goSum, Mode: 0600},
		{Path: "main.go", Data: mainGo, Mode: 0600},
	}

	manifest := capsule.Manifest{
		SchemaVersion: capsule.SchemaVersionV3,
		Execution: capsule.Execution{
			Argv: []string{
				"go", "build", "-mod=readonly",
				"-o", "/tmp/offline-build", ".",
			},
			WorkingDir: ".",
			ExitCode:   1,
		},
		Platform: capsule.Platform{
			OS:           "linux",
			Architecture: "amd64",
		},
		Runtime: &capsule.Runtime{
			Kind:    "go",
			Version: capsule.SupportedGoVersion,
		},
		GoBuildEnv: &capsule.GoBuildEnvironment{
			GOOS:       "linux",
			GOARCH:     "amd64",
			CGOEnabled: "0",
		},
		GoModules: []capsule.GoModuleV3{{
			Path:      modPath,
			Version:   modVersion,
			Artifacts: moduleArtifacts,
		}},
	}

	// Obtain the ORIGINAL failure from a real offline Docker build.
	originalWorkspace := t.TempDir()
	for _, file := range files {
		if err := os.WriteFile(
			filepath.Join(originalWorkspace, file.Path),
			file.Data,
			0600,
		); err != nil {
			t.Fatal(err)
		}
	}

	cacheRoot := t.TempDir()
	cacheDir := filepath.Join(
		cacheRoot, "cache", "download",
		"example.com", "buildfossil", "fixture", "@v",
	)
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, ext := range []string{".zip", ".mod", ".info"} {
		if err := os.WriteFile(
			filepath.Join(cacheDir, modVersion+ext),
			artifacts["module-0001"+ext],
			0600,
		); err != nil {
			t.Fatal(err)
		}
	}

	user := strconv.Itoa(os.Geteuid()) + ":" +
		strconv.Itoa(os.Getegid())

	runtimeSpec := &docker.RuntimeSpec{
		Kind:    "go",
		Version: capsule.SupportedGoVersion,
	}

	var originalStderr bytes.Buffer
	originalCode, err := docker.RunWithSDKRuntimeOfflineExec(
		ctx,
		originalWorkspace,
		manifest.Execution.Argv,
		user,
		io.Discard,
		&originalStderr,
		runtimeSpec,
		cacheRoot,
	)
	if err != nil {
		t.Fatalf("original build: %v", err)
	}
	if originalCode == 0 {
		t.Fatal("original Go build unexpectedly succeeded")
	}
	if !strings.Contains(originalStderr.String(), "undefined: missingSymbol") {
		t.Fatalf("unexpected original stderr:\n%s", originalStderr.String())
	}

	manifest.Execution.ExitCode = originalCode
	manifest.Execution.Stderr = originalStderr.String()

	var archive bytes.Buffer
	if err := capsule.WriteWorkspaceV3(
		&archive, manifest, files, artifacts,
	); err != nil {
		t.Fatalf("write schema-v3 capsule: %v", err)
	}

	capsulePath := filepath.Join(t.TempDir(), "failure.bfc")
	if err := os.WriteFile(capsulePath, archive.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}

	verified, err := capsule.ReadVerifiedV3(capsulePath)
	if err != nil {
		t.Fatalf("verify schema-v3 capsule: %v", err)
	}
	if verified.Manifest.SchemaVersion != capsule.SchemaVersionV3 {
		t.Fatal("unexpected schema version")
	}

	result, err := RunV3WithOptions(
		ctx,
		capsulePath,
		Options{AllowArbitraryCommand: true},
	)
	if err != nil {
		t.Fatalf("Replay v3: %v", err)
	}

	if result.Outcome != OutcomeReproduced {
		t.Fatalf(
			"outcome=%q, want=%q; original=%d replay=%d\noriginal stderr:\n%s",
			result.Outcome,
			OutcomeReproduced,
			result.OriginalExitCode,
			result.ReplayExitCode,
			originalStderr.String(),
		)
	}

	if result.ReplayExitCode != originalCode {
		t.Fatalf(
			"replay exit=%d, original exit=%d",
			result.ReplayExitCode,
			originalCode,
		)
	}

	t.Logf("SCHEMA V3 REPLAY: %s", result.Outcome)
	t.Logf("ORIGINAL EXIT: %d", originalCode)
	t.Logf("REPLAY EXIT: %d", result.ReplayExitCode)

	// Exercise the real CLI against the same verified capsule.
	cliBinary := filepath.Join(t.TempDir(), "buildfossil")

	buildCLI := exec.CommandContext(
		ctx,
		"go",
		"build",
		"-o",
		cliBinary,
		"../../cmd/buildfossil",
	)

	buildOutput, err := buildCLI.CombinedOutput()
	if err != nil {
		t.Fatalf(
			"build CLI: %v\n%s",
			err,
			string(buildOutput),
		)
	}

	cli := exec.CommandContext(
		ctx,
		cliBinary,
		"replay",
		"--v3",
		"--allow-command",
		capsulePath,
	)

	cliOutput, err := cli.CombinedOutput()
	if err != nil {
		t.Fatalf(
			"CLI Replay v3: %v\n%s",
			err,
			string(cliOutput),
		)
	}

	output := string(cliOutput)

	for _, expected := range []string{
		"Original exit code: 1",
		"Replay exit code:   1",
		"Outcome:            reproduced",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf(
				"CLI output missing %q:\n%s",
				expected,
				output,
			)
		}
	}

	t.Log("CLI REPLAY V3: PASS")
	t.Logf("CLI OUTPUT:\n%s", output)

}
