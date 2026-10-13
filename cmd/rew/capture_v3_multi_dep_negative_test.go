package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCaptureV3TwoDependenciesRejectsMissingZIP(t *testing.T) {
	fixture := makeCaptureV3TwoDependenciesFixture(t)

	missingZIP := filepath.Join(
		fixture.cacheRoot,
		"cache",
		"download",
		"example.com",
		"rew",
		"second",
		"@v",
		"v1.0.0.zip",
	)

	if err := os.Remove(missingZIP); err != nil {
		t.Fatalf("remove second module ZIP: %v", err)
	}

	code := captureV3FixtureCommand(fixture)
	if code != 125 {
		t.Fatalf("capture exit=%d, want 125", code)
	}

	if _, err := os.Stat(fixture.output); !os.IsNotExist(err) {
		t.Fatalf("rejected capture unexpectedly produced archive: %v", err)
	}
}

func TestCaptureV3TwoDependenciesRejectsCorruptedZIP(t *testing.T) {
	fixture := makeCaptureV3TwoDependenciesFixture(t)

	zipPath := filepath.Join(
		fixture.cacheRoot,
		"cache",
		"download",
		"example.com",
		"rew",
		"second",
		"@v",
		"v1.0.0.zip",
	)

	if err := os.WriteFile(zipPath, []byte("corrupted ZIP"), 0600); err != nil {
		t.Fatal(err)
	}

	code := captureV3FixtureCommand(fixture)
	if code != 125 {
		t.Fatalf("capture exit=%d, want 125", code)
	}

	if _, err := os.Stat(fixture.output); !os.IsNotExist(err) {
		t.Fatalf("rejected capture unexpectedly produced archive: %v", err)
	}
}

func TestCaptureV3TwoDependenciesRejectsWrongGoSum(t *testing.T) {
	fixture := makeCaptureV3TwoDependenciesFixture(t)

	goSumPath := filepath.Join(fixture.workspace, "go.sum")

	data, err := os.ReadFile(goSumPath)
	if err != nil {
		t.Fatal(err)
	}

	const original = "example.com/rew/second v1.0.0 h1:"
	if !strings.Contains(string(data), original) {
		t.Fatal("second module checksum not found")
	}

	lines := strings.Split(string(data), "\n")
	changed := false

	for i, line := range lines {
		if strings.HasPrefix(line, original) {
			lines[i] = original + strings.Repeat("A", 43) + "="
			changed = true
			break
		}
	}

	if !changed {
		t.Fatal("failed to replace second module checksum")
	}

	if err := os.WriteFile(
		goSumPath,
		[]byte(strings.Join(lines, "\n")),
		0600,
	); err != nil {
		t.Fatal(err)
	}

	code := captureV3FixtureCommand(fixture)
	if code != 125 {
		t.Fatalf("capture exit=%d, want 125", code)
	}

	if _, err := os.Stat(fixture.output); !os.IsNotExist(err) {
		t.Fatalf("rejected capture unexpectedly produced archive: %v", err)
	}
}
