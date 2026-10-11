package main

import "testing"

func TestReplayMemoryLabel(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{256 << 20, "256 MiB"},
		{512 << 20, "512 MiB"},
		{1 << 30, "1 GiB"},
		{2 << 30, "2 GiB"},
	}

	for _, tt := range tests {
		if got := replayMemoryLabel(tt.bytes); got != tt.want {
			t.Errorf("memory %d: got %q, want %q",
				tt.bytes, got, tt.want)
		}
	}
}
