package main

import (
	"bytes"
	"testing"
)

func TestReplayMemoryOutput(t *testing.T) {
	var output bytes.Buffer

	printReplayMemory(&output, 1<<30)

	want := "Replay memory:      1 GiB\n"
	if got := output.String(); got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}
