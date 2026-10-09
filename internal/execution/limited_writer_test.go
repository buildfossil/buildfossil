package execution

import (
	"strings"
	"testing"
)

func TestLimitedWriter(t *testing.T) {
	tests := []struct {
		name      string
		limit     int
		chunks    []string
		want      string
		truncated bool
	}{
		{
			name:   "below limit",
			limit:  10,
			chunks: []string{"hello"},
			want:   "hello",
		},
		{
			name:   "exact limit",
			limit:  5,
			chunks: []string{"hello"},
			want:   "hello",
		},
		{
			name:      "exceeds limit",
			limit:     5,
			chunks:    []string{"hello world"},
			want:      "hello",
			truncated: true,
		},
		{
			name:      "multiple writes",
			limit:     5,
			chunks:    []string{"abc", "def", "ghi"},
			want:      "abcde",
			truncated: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newLimitedWriter(tt.limit)

			for _, chunk := range tt.chunks {
				n, err := w.Write([]byte(chunk))
				if err != nil {
					t.Fatalf("Write() error = %v", err)
				}
				if n != len(chunk) {
					t.Fatalf("Write() = %d, want %d", n, len(chunk))
				}
			}

			if got := w.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}

			if got := w.Truncated(); got != tt.truncated {
				t.Errorf("Truncated() = %v, want %v", got, tt.truncated)
			}
		})
	}
}

func TestLimitedWriterLargeOutput(t *testing.T) {
	w := newLimitedWriter(MaxCapturedOutput)

	data := strings.Repeat("x", MaxCapturedOutput+1024)

	n, err := w.Write([]byte(data))
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if n != len(data) {
		t.Errorf("Write() = %d, want %d", n, len(data))
	}

	if got := len(w.String()); got != MaxCapturedOutput {
		t.Errorf("stored bytes = %d, want %d", got, MaxCapturedOutput)
	}

	if !w.Truncated() {
		t.Error("expected truncated output")
	}
}
