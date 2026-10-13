package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrintReplayStderrDiagnostics(t *testing.T) {
	tests := []struct {
		name        string
		original    string
		originalCut bool
		replayed    []byte
		replayedCut bool
		want        []string
	}{
		{
			name:     "different stderr",
			original: "/Users/dev/go/pkg/mod/example.go: error\n",
			replayed: []byte("/gomodcache/example.go: error\n"),
			want: []string{
				"WARNING: stderr may contain sensitive information.",
				"=== Original stderr ===",
				"/Users/dev/go/pkg/mod/example.go: error",
				"=== Replay stderr ===",
				"/gomodcache/example.go: error",
			},
		},
		{
			name: "empty stderr",
			want: []string{
				"=== Original stderr ===\n(empty)",
				"=== Replay stderr ===\n(empty)",
			},
		},
		{
			name:        "capture and replay truncated",
			original:    "original output",
			originalCut: true,
			replayed:    []byte("replay output"),
			replayedCut: true,
			want: []string{
				"original output",
				"replay output",
				"[stderr truncated during capture or replay]",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer

			printReplayStderrDiagnostics(
				&output,
				tt.original,
				tt.originalCut,
				tt.replayed,
				tt.replayedCut,
				false,
				false,
			)

			for _, expected := range tt.want {
				if !strings.Contains(output.String(), expected) {
					t.Errorf("missing %q in output:\n%s", expected, output.String())
				}
			}
		})
	}
}

func TestPrintReplayStderrDiagnosticsDisplayLimit(t *testing.T) {
	original := strings.Repeat("a", maxDisplayedStderr+100)
	replayed := []byte(strings.Repeat("b", maxDisplayedStderr+100))

	var output bytes.Buffer
	printReplayStderrDiagnostics(&output, original, false, replayed, false, false, false)

	text := output.String()

	if !strings.Contains(text, "[display truncated to 16 KiB]") {
		t.Fatal("display truncation marker missing")
	}
	if strings.Contains(text, strings.Repeat("a", maxDisplayedStderr+1)) {
		t.Fatal("original stderr exceeded display limit")
	}
	if strings.Contains(text, strings.Repeat("b", maxDisplayedStderr+1)) {
		t.Fatal("replayed stderr exceeded display limit")
	}
}

func TestPrintReplayStderrDiagnosticsPreTruncated(t *testing.T) {
	var output bytes.Buffer

	printReplayStderrDiagnostics(
		&output,
		"original",
		false,
		[]byte("replayed"),
		false,
		true,
		true,
	)

	if count := strings.Count(
		output.String(),
		"[display truncated to 16 KiB]",
	); count != 2 {
		t.Fatalf("truncation markers = %d, want 2", count)
	}
}
