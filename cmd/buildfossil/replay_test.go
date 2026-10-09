package main

import (
	"path/filepath"
	"testing"

	"github.com/buildfossil/buildfossil/internal/capsule"
)

func TestReplayInvalidArguments(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "no arguments"},
		{name: "flag without path", args: []string{"--allow-command"}},
		{name: "duplicate flag", args: []string{"--allow-command", "--allow-command"}},
		{name: "too many arguments", args: []string{"first.bfc", "second.bfc"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if code := runReplay(tt.args); code != 2 {
				t.Fatalf("expected usage exit code 2, got %d", code)
			}
		})
	}
}

func TestReplayCLIRejectsArbitraryCommandByDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.bfc")

	m := capsule.Manifest{
		SchemaVersion: 1,
		Platform: capsule.Platform{
			OS:           "linux",
			Architecture: "amd64",
		},
		Execution: capsule.Execution{
			Argv:       []string{"/bin/sh", "-c", "exit 23"},
			WorkingDir: ".",
			ExitCode:   23,
		},
	}

	err := capsule.WriteFile(path, m, capsule.WorkspaceFile{
		Path: "fixture.txt",
		Data: []byte("fixture\n"),
	})
	if err != nil {
		t.Fatalf("write capsule: %v", err)
	}

	if code := runReplay([]string{path}); code != 125 {
		t.Fatalf("expected refusal exit code 125, got %d", code)
	}
}
