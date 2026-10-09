package docker

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestSDKConnection(t *testing.T) {
	if os.Getenv("BUILDFOSSIL_DOCKER_TEST") != "1" {
		t.Skip("Docker integration test disabled")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := CheckSDKConnection(ctx); err != nil {
		t.Fatalf("Docker SDK connection failed: %v", err)
	}
}

func TestSDKRunStreamsAndExitCode(t *testing.T) {
	if os.Getenv("BUILDFOSSIL_DOCKER_TEST") != "1" {
		t.Skip("Docker integration test disabled")
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)
	defer cancel()

	workspace := t.TempDir()

	var stdout, stderr bytes.Buffer

	exitCode, err := RunWithSDK(
		ctx,
		workspace,
		[]string{
			"/bin/sh",
			"-c",
			"printf 'hello stdout\\n'; printf 'hello stderr\\n' >&2; exit 23",
		},
		fmt.Sprintf("%d:%d", os.Geteuid(), os.Getegid()),
		&stdout,
		&stderr,
	)

	if err != nil {
		t.Fatalf("RunWithSDK failed: %v", err)
	}

	if exitCode != 23 {
		t.Fatalf("exit code = %d; want 23", exitCode)
	}

	if got := stdout.String(); got != "hello stdout\n" {
		t.Errorf("stdout = %q; want %q", got, "hello stdout\n")
	}

	if got := stderr.String(); got != "hello stderr\n" {
		t.Errorf("stderr = %q; want %q", got, "hello stderr\n")
	}
}

func TestSDKRunSecurityRestrictions(t *testing.T) {
	if os.Getenv("BUILDFOSSIL_DOCKER_TEST") != "1" {
		t.Skip("Docker integration test disabled")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	workspace := t.TempDir()
	var stdout, stderr bytes.Buffer

	code, err := RunWithSDK(
		ctx,
		workspace,
		[]string{
			"/bin/sh", "-c",
			`echo "uid=$(id -u)"
			echo "gid=$(id -g)"

			if touch /buildfossil-root-write-test 2>/dev/null; then
				echo "FAIL: root filesystem writable"
				exit 11
			fi
			echo "PASS: root filesystem read-only"

			if touch /workspace/buildfossil-write-test 2>/dev/null; then
				echo "FAIL: workspace writable"
				exit 12
			fi
			echo "PASS: workspace read-only"

			if [ "$(id -u)" = "0" ]; then
				echo "FAIL: running as root"
				exit 13
			fi
			echo "PASS: non-root user"`,
		},
		fmt.Sprintf("%d:%d", os.Geteuid(), os.Getegid()),
		&stdout,
		&stderr,
	)
	if err != nil {
		t.Fatalf("SDK execution failed: %v", err)
	}

	if code != 0 {
		t.Fatalf(
			"security checks failed: exit=%d stdout=%q stderr=%q",
			code, stdout.String(), stderr.String(),
		)
	}

	t.Logf("Security diagnostics:\n%s", stdout.String())

	if !strings.Contains(stdout.String(), "PASS: root filesystem read-only") ||
		!strings.Contains(stdout.String(), "PASS: workspace read-only") ||
		!strings.Contains(stdout.String(), "PASS: non-root user") {
		t.Fatalf("security diagnostics incomplete: %q", stdout.String())
	}
}

func TestSDKRunFastCommandDoesNotLoseOutput(t *testing.T) {
	if os.Getenv("BUILDFOSSIL_DOCKER_TEST") != "1" {
		t.Skip("Docker integration test disabled")
	}

	workspace := t.TempDir()
	user := fmt.Sprintf("%d:%d", os.Geteuid(), os.Getegid())

	for i := 0; i < 30; i++ {
		ctx, cancel := context.WithTimeout(
			context.Background(),
			10*time.Second,
		)

		var stdout, stderr bytes.Buffer

		code, err := RunWithSDK(
			ctx,
			workspace,
			[]string{
				"/bin/sh",
				"-c",
				"printf 'FAST_STDOUT'; printf 'FAST_STDERR' >&2; exit 23",
			},
			user,
			&stdout,
			&stderr,
		)

		cancel()

		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}

		if code != 23 {
			t.Fatalf("iteration %d: exit=%d; want 23", i, code)
		}

		if stdout.String() != "FAST_STDOUT" {
			t.Fatalf(
				"iteration %d: stdout=%q",
				i, stdout.String(),
			)
		}

		if stderr.String() != "FAST_STDERR" {
			t.Fatalf(
				"iteration %d: stderr=%q",
				i, stderr.String(),
			)
		}
	}
}

func TestSDKRunLargeStderr(t *testing.T) {
	if os.Getenv("BUILDFOSSIL_DOCKER_TEST") != "1" {
		t.Skip("Docker integration test disabled")
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)
	defer cancel()

	workspace := t.TempDir()
	user := fmt.Sprintf("%d:%d", os.Geteuid(), os.Getegid())

	var stdout, stderr bytes.Buffer

	code, err := RunWithSDK(
		ctx,
		workspace,
		[]string{
			"/bin/sh",
			"-c",
			"head -c 2097152 /dev/zero | tr '\\000' 'A' >&2; exit 23",
		},
		user,
		&stdout,
		&stderr,
	)
	if err != nil {
		t.Fatalf("RunWithSDK: %v", err)
	}

	if code != 23 {
		t.Fatalf("exit code = %d; want 23", code)
	}

	const expectedSize = 2 * 1024 * 1024

	if stderr.Len() != expectedSize {
		t.Fatalf(
			"stderr length = %d; want %d",
			stderr.Len(),
			expectedSize,
		)
	}

	if stdout.Len() != 0 {
		t.Fatalf("unexpected stdout length: %d", stdout.Len())
	}

	for i, b := range stderr.Bytes() {
		if b != 'A' {
			t.Fatalf("unexpected stderr byte at offset %d: %q", i, b)
		}
	}
}

func TestSDKRunStartFailure(t *testing.T) {
	if os.Getenv("BUILDFOSSIL_DOCKER_TEST") != "1" {
		t.Skip("Docker integration test disabled")
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	workspace := t.TempDir()
	user := fmt.Sprintf("%d:%d", os.Geteuid(), os.Getegid())

	var stdout, stderr bytes.Buffer

	_, err := RunWithSDK(
		ctx,
		workspace,
		[]string{"/buildfossil-command-does-not-exist"},
		user,
		&stdout,
		&stderr,
	)

	if err == nil {
		t.Fatal("expected Docker start failure")
	}

	if !strings.Contains(err.Error(), "docker SDK: start container:") {
		t.Fatalf("unexpected error category: %v", err)
	}
}
