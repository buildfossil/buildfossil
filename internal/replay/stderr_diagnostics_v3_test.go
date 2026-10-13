package replay

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewStderrDiagnosticsV3(t *testing.T) {
	t.Run("short output", func(t *testing.T) {
		d := newStderrDiagnosticsV3(
			"original error",
			false,
			[]byte("replay error"),
			false,
		)

		if d.Original != "original error" ||
			string(d.Replayed) != "replay error" {
			t.Fatal("diagnostic content mismatch")
		}
		if d.OriginalDisplayTruncated || d.ReplayDisplayTruncated {
			t.Fatal("short diagnostics must not be truncated")
		}
	})

	t.Run("display limits", func(t *testing.T) {
		original := strings.Repeat("a", maxStderrDiagnosticsV3+100)
		replayed := bytes.Repeat([]byte("b"), maxStderrDiagnosticsV3+100)

		d := newStderrDiagnosticsV3(original, false, replayed, false)

		if len(d.Original) != maxStderrDiagnosticsV3 {
			t.Fatalf("original length = %d", len(d.Original))
		}
		if len(d.Replayed) != maxStderrDiagnosticsV3 {
			t.Fatalf("replayed length = %d", len(d.Replayed))
		}
		if !d.OriginalDisplayTruncated || !d.ReplayDisplayTruncated {
			t.Fatal("display truncation flags missing")
		}
	})

	t.Run("capture truncation remains separate", func(t *testing.T) {
		d := newStderrDiagnosticsV3(
			"error",
			true,
			[]byte("error"),
			true,
		)

		if !d.OriginalTruncated || !d.ReplayTruncated {
			t.Fatal("capture truncation flags lost")
		}
		if d.OriginalDisplayTruncated || d.ReplayDisplayTruncated {
			t.Fatal("unexpected display truncation")
		}
	})

	t.Run("replayed bytes are copied", func(t *testing.T) {
		source := []byte("docker error")
		d := newStderrDiagnosticsV3("", false, source, false)

		source[0] = 'X'

		if string(d.Replayed) != "docker error" {
			t.Fatal("diagnostics share mutable replay buffer")
		}
	})
}
