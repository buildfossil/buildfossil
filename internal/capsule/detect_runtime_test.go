package capsule

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestDetectRuntimeRejectsUnsupportedCommands(t *testing.T) {
	tests := []struct {
		name string
		argv []string
	}{
		{"empty", nil},
		{"echo", []string{"echo", "hello"}},
		{"go test", []string{"go", "test", "./..."}},
		{"go run", []string{"go", "run", "."}},
		{"shell wrapper", []string{"sh", "-c", "go build ./..."}},
		{"extra arguments", []string{"go", "build", "./...", "-v"}},
		{"absolute path", []string{"/usr/local/go/bin/go", "build", "./..."}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectRuntime(tt.argv, t.TempDir())

			if got != nil {
				t.Fatalf("expected nil runtime, got %+v", got)
			}
		})
	}
}

func TestDetectRuntimeGoBuild(t *testing.T) {
	output, err := exec.Command("go", "version").Output()
	if err != nil {
		t.Skipf("Go toolchain unavailable: %v", err)
	}

	if !strings.Contains(string(output), "go"+SupportedGoVersion+" ") {
		t.Skipf("unsupported local Go version: %s", output)
	}

	got := DetectRuntime(
		[]string{"go", "build", "./..."},
		t.TempDir(),
	)

	if got == nil {
		t.Fatal("expected Go runtime")
	}

	if got.Kind != "go" {
		t.Fatalf("kind = %q, want go", got.Kind)
	}

	if got.Version != SupportedGoVersion {
		t.Fatalf(
			"version = %q, want %q",
			got.Version,
			SupportedGoVersion,
		)
	}
}

func TestDetectRuntimeMissingGoExecutable(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	got := DetectRuntime(
		[]string{"go", "build", "./..."},
		t.TempDir(),
	)

	if got != nil {
		t.Fatalf("expected nil without Go executable, got %+v", got)
	}
}

func TestDetectRuntimeUnsupportedGoVersion(t *testing.T) {
	dir := t.TempDir()

	script := "#!/bin/sh\nprintf 'go version go1.25.0 linux/amd64\\n'\n"

	if err := os.WriteFile(
		dir+"/go",
		[]byte(script),
		0700,
	); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", dir)

	got := DetectRuntime(
		[]string{"go", "build", "./..."},
		t.TempDir(),
	)

	if got != nil {
		t.Fatalf("expected nil for unsupported version, got %+v", got)
	}
}
