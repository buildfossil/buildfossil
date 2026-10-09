package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/buildfossil/buildfossil/internal/capsule"
)

func TestCaptureSuccessDoesNotCreateCapsule(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "failure.bfc")

	code := captureCommand(
		[]string{"/bin/sh", "-c", "exit 0"},
		output,
		false,
	)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	_, err := os.Stat(output)
	if !os.IsNotExist(err) {
		t.Fatalf("expected no capsule, stat error: %v", err)
	}
}

func TestCaptureFailureCreatesVerifiedCapsule(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "failure.bfc")

	fixture := filepath.Join(dir, "fixture.txt")

	if err := os.WriteFile(fixture, []byte("test fixture\n"), 0600); err != nil {
		t.Fatalf("create fixture: %v", err)
	}

	code := captureCommandInWorkspace(
		[]string{"/bin/sh", "-c", "echo CAPTURE_FAILED >&2; exit 23"},
		dir,
		output,
		false,
	)

	if code != 23 {
		t.Fatalf("expected exit code 23, got %d", code)
	}

	verified, err := capsule.ReadVerified(output)
	if err != nil {
		t.Fatalf("read verified capsule: %v", err)
	}

	exec := verified.Manifest.Execution

	if exec.ExitCode != 23 {
		t.Fatalf("expected recorded exit code 23, got %d", exec.ExitCode)
	}

	if exec.Stderr != "CAPTURE_FAILED\n" {
		t.Fatalf("unexpected recorded stderr: %q", exec.Stderr)
	}

	if len(verified.Manifest.Workspace.Files) != 1 {
		t.Fatalf("expected one workspace file")
	}
}

func TestCaptureDoesNotOverwriteExistingCapsule(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "failure.bfc")

	original := []byte("existing capsule must survive")

	if err := os.WriteFile(output, original, 0600); err != nil {
		t.Fatalf("create existing capsule: %v", err)
	}

	if err := os.WriteFile(
		filepath.Join(dir, "fixture.txt"),
		[]byte("fixture\n"),
		0600,
	); err != nil {
		t.Fatalf("create fixture: %v", err)
	}

	code := captureCommandInWorkspace(
		[]string{"/bin/sh", "-c", "exit 23"},
		dir,
		output,
		false,
	)

	if code != 125 {
		t.Fatalf("expected capture error 125, got %d", code)
	}

	actual, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("read existing capsule: %v", err)
	}

	if string(actual) != string(original) {
		t.Fatal("existing capsule was modified")
	}
}

func TestCaptureDemoModeSavesOnSuccess(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "success.bfc")

	fixture := filepath.Join(dir, "fixture.txt")
	if err := os.WriteFile(fixture, []byte("test fixture\n"), 0600); err != nil {
		t.Fatalf("create fixture: %v", err)
	}

	code := captureCommandInWorkspace(
		[]string{"/bin/sh", "-c", "exit 0"},
		dir,
		output,
		true,
	)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	verified, err := capsule.ReadVerified(output)
	if err != nil {
		t.Fatalf("read verified capsule: %v", err)
	}

	if verified.Manifest.Execution.ExitCode != 0 {
		t.Fatalf(
			"expected recorded exit code 0, got %d",
			verified.Manifest.Execution.ExitCode,
		)
	}
}
