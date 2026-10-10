package docker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestSDKOfflineExecRunner(t *testing.T) {
	if os.Getenv("BUILDFOSSIL_DOCKER_TEST") != "1" {
		t.Skip("Docker integration test disabled")
	}

	fixture := makeOfflineGoFixture(t)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		180*time.Second,
	)
	defer cancel()

	runtime := &RuntimeSpec{
		Kind:    "go",
		Version: "1.26.6",
	}

	user := fmt.Sprintf("%d:%d", os.Geteuid(), os.Getegid())

	t.Run("offline Go build", func(t *testing.T) {
		var stdout, stderr bytes.Buffer

		code, err := RunWithSDKRuntimeOfflineExec(
			ctx,
			fixture.workspace,
			[]string{
				"go",
				"build",
				"-mod=readonly",
				"-o",
				"/tmp/offline-build",
				".",
			},
			user,
			&stdout,
			&stderr,
			runtime,
			fixture.cacheRoot,
		)
		if err != nil {
			t.Fatalf(
				"offline build execution: %v\nstderr:\n%s",
				err,
				stderr.String(),
			)
		}

		if code != 0 {
			t.Fatalf(
				"offline build exit=%d\nstderr:\n%s",
				code,
				stderr.String(),
			)
		}

		if strings.Contains(stderr.String(), "module lookup disabled") {
			t.Fatalf("Go attempted an unavailable module lookup:\n%s", stderr.String())
		}

		t.Log("OFFLINE GO BUILD: PASS")
	})

	t.Run("stderr and exit code", func(t *testing.T) {
		var stdout, stderr bytes.Buffer

		code, err := RunWithSDKRuntimeOfflineExec(
			ctx,
			fixture.workspace,
			[]string{
				"/bin/sh",
				"-c",
				"printf 'buildfossil-test-error\n' >&2; exit 23",
			},
			user,
			&stdout,
			&stderr,
			runtime,
			fixture.cacheRoot,
		)
		if err != nil {
			t.Fatalf("failure execution: %v", err)
		}

		if code != 23 {
			t.Fatalf("exit code=%d, want 23", code)
		}

		if stderr.String() != "buildfossil-test-error\n" {
			t.Fatalf("stderr=%q, want expected error", stderr.String())
		}

		t.Log("STDERR + EXIT CODE: PASS")
	})
}

func TestSDKOfflineExecCancellation(t *testing.T) {
	if os.Getenv("BUILDFOSSIL_DOCKER_TEST") != "1" {
		t.Skip("Docker integration test disabled")
	}

	fixture := makeOfflineGoFixture(t)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		3*time.Second,
	)
	defer cancel()

	var stdout, stderr bytes.Buffer

	start := time.Now()

	_, err := RunWithSDKRuntimeOfflineExec(
		ctx,
		fixture.workspace,
		[]string{
			"/bin/sh",
			"-c",
			"sleep 60",
		},
		fmt.Sprintf("%d:%d", os.Geteuid(), os.Getegid()),
		&stdout,
		&stderr,
		&RuntimeSpec{
			Kind:    "go",
			Version: "1.26.6",
		},
		fixture.cacheRoot,
	)

	if err == nil {
		t.Fatal("expected cancellation error")
	}

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got: %v", err)
	}

	if elapsed := time.Since(start); elapsed > 15*time.Second {
		t.Fatalf("cancellation too slow: %s", elapsed)
	}

	t.Logf("CANCELLATION: PASS (%s)", time.Since(start))
}
