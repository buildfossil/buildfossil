package main

import "testing"

func TestReplayShowStderrV3RejectsInvalidArguments(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "missing allow command",
			args: []string{
				"--v3", "--show-stderr", "failure.bfc",
			},
		},
		{
			name: "unsupported v2",
			args: []string{
				"--v2", "--allow-command", "--show-stderr", "failure.bfc",
			},
		},
		{
			name: "missing capsule",
			args: []string{
				"--v3", "--allow-command", "--show-stderr",
			},
		},
		{
			name: "duplicate flag",
			args: []string{
				"--v3", "--allow-command",
				"--show-stderr", "--show-stderr", "failure.bfc",
			},
		},
		{
			name: "invalid memory",
			args: []string{
				"--v3", "--allow-command",
				"--memory", "3g", "--show-stderr", "failure.bfc",
			},
		},
		{
			name: "missing memory value",
			args: []string{
				"--v3", "--allow-command",
				"--memory", "--show-stderr", "failure.bfc",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if code := runReplay(tt.args); code != 2 {
				t.Fatalf("runReplay returned %d, want 2", code)
			}
		})
	}
}
