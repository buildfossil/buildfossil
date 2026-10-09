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
