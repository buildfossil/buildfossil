package execution

import (
	"bytes"
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
