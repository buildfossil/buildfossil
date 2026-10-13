package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestSplashPlainOutput(t *testing.T) {
	var out bytes.Buffer
	writeSplash(&out, "dev", false)

	text := out.String()

	if strings.Contains(text, "\x1b[") {
		t.Fatal("plain splash contains ANSI sequences")
	}

	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	if len(lines) != 7 {
		t.Fatalf("got %d lines, want 7", len(lines))
	}

	if !strings.Contains(text, "Capture failed Go builds. Replay them offline in Docker.") {
		t.Fatal("missing tagline")
	}

	if !strings.Contains(text, "Version: dev | Engine: Docker") {
		t.Fatal("missing version and engine")
	}
}

func TestSplashTrueColor(t *testing.T) {
	var out bytes.Buffer
	writeSplash(&out, "v0.2.0-alpha.1", true)

	text := out.String()

	for _, code := range []string{
		splashPurple,
		splashTeal,
		splashEmerald,
		splashReset,
	} {
		if !strings.Contains(text, code) {
			t.Fatalf("missing color sequence %q", code)
		}
	}

	if strings.Count(text, splashReset) != 5 {
		t.Fatal("expected one color reset per banner line")
	}
}

func TestSplashColorDisabled(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	if splashSupportsColor(nil) {
		t.Fatal("NO_COLOR must disable colors")
	}
}

func TestSplashCommand(t *testing.T) {
	code, stdout, stderr := runHelpTest([]string{"splash"})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	if !strings.Contains(stdout, "Engine: Docker") {
		t.Fatal("splash command missing engine metadata")
	}

	if stderr != "" {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
}

func TestSplashRejectsExtraArguments(t *testing.T) {
	code, _, stderr := runHelpTest([]string{"splash", "extra"})

	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}

	if !strings.Contains(stderr, "Usage: rew splash") {
		t.Fatalf("unexpected error: %q", stderr)
	}
}
