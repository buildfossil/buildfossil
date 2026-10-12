package main

import "testing"

func TestCaptureV3GoEmbedCLIInvalidArguments(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{
			name: "duplicate embed flag",
			args: []string{
				"capture", "--v3",
				"--include", "go.mod",
				"--include-go-embed",
				"--include-go-embed",
				"--", "go", "build", "./...",
			},
		},
		{
			name: "missing include value",
			args: []string{
				"capture", "--v3",
				"--include",
				"--include-go-embed",
				"--", "go", "build", "./...",
			},
		},
		{
			name: "no explicit includes",
			args: []string{
				"capture", "--v3",
				"--include-go-embed",
				"--", "go", "build", "./...",
			},
		},
		{
			name: "missing separator",
			args: []string{
				"capture", "--v3",
				"--include", "go.mod",
				"--include-go-embed",
				"go", "build", "./...",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := run(tc.args); got != 2 {
				t.Fatalf("run() = %d, want 2", got)
			}
		})
	}
}
