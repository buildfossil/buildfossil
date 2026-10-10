package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"

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
		{
			name: "diagnostics without v2",
			args: []string{
				"--allow-command",
				"--diagnose-dependencies",
				"failure.bfc",
			},
		},
		{
			name: "diagnostics without allow-command",
			args: []string{
				"--v2",
				"--diagnose-dependencies",
				"failure.bfc",
			},
		},
		{
			name: "diagnostics missing capsule",
			args: []string{
				"--v2",
				"--allow-command",
				"--diagnose-dependencies",
			},
		},
		{
			name: "diagnostics wrong flag order",
			args: []string{
				"--diagnose-dependencies",
				"--v2",
				"--allow-command",
				"failure.bfc",
			},
		},
		{
			name: "diagnostics duplicate flag",
			args: []string{
				"--v2",
				"--allow-command",
				"--diagnose-dependencies",
				"--diagnose-dependencies",
				"failure.bfc",
			},
		},
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

func TestReplayContextTimeout(t *testing.T) {
	tests := []struct {
		name         string
		useV2        bool
		wantDeadline bool
	}{
		{
			name:         "v1 has 30 second timeout",
			useV2:        false,
			wantDeadline: true,
		},
		{
			name:         "v2 has no global timeout",
			useV2:        true,
			wantDeadline: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			ctx, cancel := replayContext(
				context.Background(),
				tt.useV2,
			)
			defer cancel()

			deadline, ok := ctx.Deadline()

			if ok != tt.wantDeadline {
				t.Fatalf(
					"deadline present = %t; want %t",
					ok,
					tt.wantDeadline,
				)
			}

			if !tt.wantDeadline {
				return
			}

			remaining := deadline.Sub(start)

			if remaining < 29*time.Second ||
				remaining > 31*time.Second {
				t.Fatalf(
					"unexpected v1 timeout: %s",
					remaining,
				)
			}
		})
	}
}
