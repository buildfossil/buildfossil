package docker

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

func TestSnapshotExecutorEndToEnd(t *testing.T) {
	if os.Getenv("BUILDFOSSIL_DOCKER_TEST") != "1" {
		t.Skip("Docker integration test disabled")
	}

	workspaceTAR, moduleTAR := makeSnapshotGoArchives(t)

	runtime := &RuntimeSpec{
		Kind:    "go",
		Version: "1.26.6",
	}

	t.Run("offline build", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(
			context.Background(), 180*time.Second,
		)
		defer cancel()

		var stdout, stderr bytes.Buffer

		code, err := RunWithSDKRuntimeSnapshotExec(
			ctx,
			workspaceTAR,
			moduleTAR,
			[]string{
				"go", "build", "-mod=readonly",
				"-o", "/tmp/offline-build", ".",
			},
			"10001:10001",
			&stdout,
			&stderr,
			runtime,
		)
		if err != nil {
			t.Fatalf("snapshot execution: %v; stderr: %s", err, stderr.String())
		}
		if code != 0 {
			t.Fatalf("build exit=%d; stderr=%s", code, stderr.String())
		}

		t.Log("SNAPSHOT EXECUTOR OFFLINE BUILD: PASS")
	})

	t.Run("stderr and exit code", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(
			context.Background(), 180*time.Second,
		)
		defer cancel()

		var stdout, stderr bytes.Buffer

		code, err := RunWithSDKRuntimeSnapshotExec(
			ctx,
			workspaceTAR,
			moduleTAR,
			[]string{
				"/bin/sh", "-c",
				"printf 'snapshot-exec-error\n' >&2; exit 23",
			},
			"10001:10001",
			&stdout,
			&stderr,
			runtime,
		)
		if err != nil {
			t.Fatalf("snapshot failure execution: %v", err)
		}
		if code != 23 {
			t.Fatalf("exit code=%d, want 23", code)
		}
		if stdout.Len() != 0 {
			t.Fatalf("unexpected stdout: %q", stdout.String())
		}
		if stderr.String() != "snapshot-exec-error\n" {
			t.Fatalf("unexpected stderr: %q", stderr.String())
		}

		t.Log("SNAPSHOT EXECUTOR STDERR + EXIT: PASS")
	})

	t.Run("cancellation", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(
			context.Background(), 3*time.Second,
		)
		defer cancel()

		var stdout, stderr bytes.Buffer

		_, err := RunWithSDKRuntimeSnapshotExec(
			ctx,
			workspaceTAR,
			moduleTAR,
			[]string{"/bin/sh", "-c", "sleep 60"},
			"10001:10001",
			&stdout,
			&stderr,
			runtime,
		)
		if err == nil {
			t.Fatal("expected cancellation error")
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected deadline exceeded, got: %v", err)
		}

		t.Log("SNAPSHOT EXECUTOR CANCELLATION: PASS")
	})

	t.Run("reject root", func(t *testing.T) {
		_, err := RunWithSDKRuntimeSnapshotExec(
			context.Background(),
			workspaceTAR,
			moduleTAR,
			[]string{"go", "version"},
			"0:0",
			nil,
			nil,
			runtime,
		)
		if err == nil {
			t.Fatal("root execution accepted")
		}
	})

	t.Run("reject empty command", func(t *testing.T) {
		_, err := RunWithSDKRuntimeSnapshotExec(
			context.Background(),
			workspaceTAR,
			moduleTAR,
			nil,
			"10001:10001",
			nil,
			nil,
			runtime,
		)
		if err == nil {
			t.Fatal("empty command accepted")
		}
	})
}
