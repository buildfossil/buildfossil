package execution

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRunSuccess(t *testing.T) {
	var stdout, stderr bytes.Buffer

	result, err := Run(
		[]string{"/bin/echo", "hello"},
		&stdout,
		&stderr,
	)

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("exit code = %d, want 0", result.ExitCode)
	}

	if got := stdout.String(); got != "hello\n" {
		t.Errorf("stdout = %q, want %q", got, "hello\n")
	}
}

func TestRunNonZeroExit(t *testing.T) {
	var stdout, stderr bytes.Buffer

	result, err := Run(
		[]string{"/bin/sh", "-c", "exit 17"},
		&stdout,
		&stderr,
	)

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result.ExitCode != 17 {
		t.Errorf("exit code = %d, want 17", result.ExitCode)
	}
}

func TestRunMissingCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer

	_, err := Run(
		[]string{"/buildfossil/nonexistent-command"},
		&stdout,
		&stderr,
	)

	if err == nil {
		t.Fatal("expected execution error, got nil")
	}
}

func TestRunEmptyCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer

	_, err := Run(nil, &stdout, &stderr)

	if err == nil {
		t.Fatal("expected error for empty command")
	}
}

func TestRunCapturesExecutionRecord(t *testing.T) {
	argv := []string{
		"/bin/sh",
		"-c",
		"printf 'hello'; printf 'failure\n' >&2; exit 17",
	}

	var stdout, stderr bytes.Buffer

	result, err := Run(argv, &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result.ExitCode != 17 {
		t.Errorf("exit code = %d, want 17", result.ExitCode)
	}

	if result.Stdout != "hello" {
		t.Errorf("recorded stdout = %q, want hello", result.Stdout)
	}

	if result.Stderr != "failure\n" {
		t.Errorf("recorded stderr = %q, want failure newline", result.Stderr)
	}

	if stdout.String() != result.Stdout {
		t.Errorf("terminal stdout differs from recorded stdout")
	}

	if stderr.String() != result.Stderr {
		t.Errorf("terminal stderr differs from recorded stderr")
	}

	if !reflect.DeepEqual(result.Argv, argv) {
		t.Errorf("argv = %q, want %q", result.Argv, argv)
	}
}

func TestRunTruncatesLargeOutput(t *testing.T) {
	var stdout, stderr bytes.Buffer

	result, err := Run(
		[]string{
			"/bin/sh",
			"-c",
			"yes x | head -c 1100000",
		},
		&stdout,
		&stderr,
	)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, want 0", result.ExitCode)
	}

	if got := len(result.Stdout); got != MaxCapturedOutput {
		t.Errorf("captured bytes = %d, want %d", got, MaxCapturedOutput)
	}

	if !result.StdoutTruncated {
		t.Error("expected StdoutTruncated = true")
	}

	if result.StderrTruncated {
		t.Error("expected StderrTruncated = false")
	}

	if stdout.Len() <= MaxCapturedOutput {
		t.Errorf("terminal output size = %d, want > %d",
			stdout.Len(), MaxCapturedOutput)
	}
}

func TestRunInDirWithExecutablePreservesArgv(t *testing.T) {
	dir := t.TempDir()
	executable := filepath.Join(dir, "custom-go")

	script := "#!/bin/sh\nprintf 'custom toolchain\\n' >&2\nexit 17\n"
	if err := os.WriteFile(executable, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}

	argv := []string{"go", "build", "./..."}
	var stdout, stderr bytes.Buffer

	result, err := RunInDirWithExecutable(
		argv, executable, dir, &stdout, &stderr,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 17 {
		t.Fatalf("exit code = %d, want 17", result.ExitCode)
	}
	if result.Stderr != "custom toolchain\n" {
		t.Fatalf("stderr = %q", result.Stderr)
	}
	if !reflect.DeepEqual(result.Argv, argv) {
		t.Fatalf("argv = %q, want %q", result.Argv, argv)
	}
}

func TestRunInDirWithExecutableRejectsEmptyExecutable(t *testing.T) {
	var stdout, stderr bytes.Buffer
	_, err := RunInDirWithExecutable(
		[]string{"go", "build", "./..."},
		"",
		t.TempDir(),
		&stdout,
		&stderr,
	)
	if err == nil {
		t.Fatal("expected empty executable error")
	}
}

func TestRunInDirWithExecutableEnvOverridesGoToolchain(t *testing.T) {
	dir := t.TempDir()
	executable := filepath.Join(dir, "fake-go")

	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$GOTOOLCHAIN\" >&2\n" +
		"exit 17\n"

	if err := os.WriteFile(executable, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}

	t.Setenv("GOTOOLCHAIN", "auto")

	argv := []string{"go", "build", "./..."}
	env := append(os.Environ(), "GOTOOLCHAIN=local")

	var stdout, stderr bytes.Buffer

	result, err := RunInDirWithExecutableEnv(
		argv, executable, dir, env, &stdout, &stderr,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 17 {
		t.Fatalf("exit code = %d, want 17", result.ExitCode)
	}
	if result.Stderr != "local\n" {
		t.Fatalf("stderr = %q, want local", result.Stderr)
	}
	if !reflect.DeepEqual(result.Argv, argv) {
		t.Fatalf("argv = %q, want %q", result.Argv, argv)
	}
}

func TestRunInDirWithExecutableEnvRejectsNilEnvironment(t *testing.T) {
	var stdout, stderr bytes.Buffer

	_, err := RunInDirWithExecutableEnv(
		[]string{"go", "build", "./..."},
		"/bin/echo",
		t.TempDir(),
		nil,
		&stdout,
		&stderr,
	)
	if err == nil {
		t.Fatal("expected nil environment error")
	}
}
