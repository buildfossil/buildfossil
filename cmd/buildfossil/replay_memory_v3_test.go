package main

import (
	"testing"
)

func TestReplayV3MemoryInvalidArguments(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "invalid memory",
			args: []string{
				"--v3", "--allow-command", "--memory", "3g", "failure.bfc",
			},
		},
		{
			name: "zero memory",
			args: []string{
				"--v3", "--allow-command", "--memory", "0", "failure.bfc",
			},
		},
		{
			name: "missing memory value",
			args: []string{
				"--v3", "--allow-command", "--memory",
			},
		},
		{
			name: "missing capsule",
			args: []string{
				"--v3", "--allow-command", "--memory", "1g",
			},
		},
		{
			name: "wrong flag order",
			args: []string{
				"--v3", "--memory", "1g", "--allow-command", "failure.bfc",
			},
		},
		{
			name: "memory on v2",
			args: []string{
				"--v2", "--allow-command", "--memory", "1g", "failure.bfc",
			},
		},
		{
			name: "duplicate memory",
			args: []string{
				"--v3", "--allow-command",
				"--memory", "1g",
				"--memory", "2g",
				"failure.bfc",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := runReplay(tt.args); got != 2 {
				t.Fatalf("exit code = %d; want 2", got)
			}
		})
	}
}
