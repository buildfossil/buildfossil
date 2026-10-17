package main

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestCleanupInvalidArguments(t *testing.T) {
	tests := [][]string{
		{"cleanup"},
		{"cleanup", "--delete"},
		{"cleanup", "--dry-run", "--delete"},
		{"cleanup", "--dry-run", "extra"},
	}

	for _, args := range tests {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			if got := run(args); got != 2 {
				t.Fatalf("run(%q) = %d, want 2", args, got)
			}
		})
	}
}

func TestCleanupAppearsInHelp(t *testing.T) {
	oldStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer func() { os.Stdout = oldStdout }()

	os.Stdout = writer
	code := run([]string{"help"})
	os.Stdout = oldStdout
	writer.Close()

	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Fatalf("help exit code = %d", code)
	}
	if !strings.Contains(string(output), "cleanup") {
		t.Fatal("cleanup missing from general help")
	}
}
