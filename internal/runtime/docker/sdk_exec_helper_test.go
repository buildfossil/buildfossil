package docker

import (
	"bytes"
	"context"
	"errors"
	"testing"
)

func TestLimitedExecBuffer(t *testing.T) {
	t.Run("within limit", func(t *testing.T) {
		var buffer limitedExecBuffer

		input := []byte("buildfossil-error\n")

		n, err := buffer.Write(input)
		if err != nil {
			t.Fatal(err)
		}

		if n != len(input) {
			t.Fatalf("written=%d, want=%d", n, len(input))
		}

		if buffer.truncated {
			t.Fatal("short output marked truncated")
		}

		if !bytes.Equal(buffer.Bytes(), input) {
			t.Fatal("output content mismatch")
		}
	})

	t.Run("exceeds limit", func(t *testing.T) {
		var buffer limitedExecBuffer

		input := bytes.Repeat(
			[]byte("x"),
			maxExecOutputBytes+100,
		)

		n, err := buffer.Write(input)
		if err != nil {
			t.Fatal(err)
		}

		if n != len(input) {
			t.Fatalf("written=%d, want=%d", n, len(input))
		}

		if !buffer.truncated {
			t.Fatal("oversized output not marked truncated")
		}

		if buffer.Len() != maxExecOutputBytes {
			t.Fatalf(
				"stored=%d, want=%d",
				buffer.Len(),
				maxExecOutputBytes,
			)
		}
	})

	t.Run("multiple writes", func(t *testing.T) {
		var buffer limitedExecBuffer

		first := bytes.Repeat(
			[]byte("a"),
			maxExecOutputBytes-10,
		)

		second := bytes.Repeat([]byte("b"), 20)

		if _, err := buffer.Write(first); err != nil {
			t.Fatal(err)
		}

		if _, err := buffer.Write(second); err != nil {
			t.Fatal(err)
		}

		if !buffer.truncated {
			t.Fatal("expected truncation")
		}

		if buffer.Len() != maxExecOutputBytes {
			t.Fatalf(
				"stored=%d, want=%d",
				buffer.Len(),
				maxExecOutputBytes,
			)
		}

		if !bytes.Equal(
			buffer.Bytes()[maxExecOutputBytes-10:],
			bytes.Repeat([]byte("b"), 10),
		) {
			t.Fatal("incorrect output prefix retained")
		}
	})
}

func TestExecuteInContainerRejectsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, _, err := executeInContainer(
		ctx,
		nil,
		"unused-container",
		"501:20",
		[]string{"go", "version"},
	)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}
