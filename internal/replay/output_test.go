package replay

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/buildfossil/buildfossil/internal/runtime/docker"
)

func TestBoundedOutputTruncation(t *testing.T) {
	var output boundedOutput

	data := bytes.Repeat([]byte("A"), 2*maxReplayOutput)

	n, err := output.Write(data)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	if n != len(data) {
		t.Fatalf("written = %d; want %d", n, len(data))
	}

	if got := len(output.Bytes()); got != maxReplayOutput {
		t.Fatalf("stored = %d; want %d", got, maxReplayOutput)
	}

	if !output.Truncated() {
		t.Fatal("expected truncated output")
	}

	outcome, err := CompareFailure(
		23,
		"original failure",
		false,
		23,
		output.Bytes(),
		output.Truncated(),
	)
	if err != nil {
		t.Fatalf("CompareFailure: %v", err)
	}

	if outcome != OutcomeInconclusive {
		t.Fatalf("outcome = %q; want inconclusive", outcome)
	}
}

func TestReplayOutputTruncationWithDocker(t *testing.T) {
	if os.Getenv("BUILDFOSSIL_DOCKER_TEST") != "1" {
		t.Skip("Docker integration test disabled")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	workspace := t.TempDir()
	user := fmt.Sprintf("%d:%d", os.Geteuid(), os.Getegid())

	var stderr boundedOutput

	code, err := docker.RunWithSDK(
		ctx,
		workspace,
		[]string{
			"/bin/sh",
			"-c",
			"head -c 2097152 /dev/zero | tr '\\000' 'A' >&2; exit 23",
		},
		user,
		io.Discard,
		&stderr,
	)
	if err != nil {
		t.Fatalf("Docker SDK execution: %v", err)
	}

	if code != 23 {
		t.Fatalf("exit code = %d; want 23", code)
	}

	if !stderr.Truncated() {
		t.Fatal("expected stderr truncation")
	}

	if len(stderr.Bytes()) != maxReplayOutput {
		t.Fatalf(
			"stored stderr = %d; want %d",
			len(stderr.Bytes()),
			maxReplayOutput,
		)
	}

	outcome, err := CompareFailure(
		23,
		"original failure",
		false,
		code,
		stderr.Bytes(),
		stderr.Truncated(),
	)
	if err != nil {
		t.Fatalf("CompareFailure: %v", err)
	}

	if outcome != OutcomeInconclusive {
		t.Fatalf(
			"outcome = %q; want %q",
			outcome,
			OutcomeInconclusive,
		)
	}
}
