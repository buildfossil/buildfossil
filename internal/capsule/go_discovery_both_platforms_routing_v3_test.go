package capsule

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoDiscoveryRoutesBothPlatformsV3(t *testing.T) {
	realGo, err := exec.LookPath("go")
	if err != nil {
		t.Skipf("Go executable unavailable: %v", err)
	}

	realGo, err = filepath.Abs(realGo)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		run  func(root, cache, executable string) error
	}{
		{
			name: "sources",
			run: func(root, cache, executable string) error {
				_, err := DiscoverGoSourceFilesV3WithExecutable(
					root, cache, executable,
					GoBuildEnvironment{GOOS: "darwin", GOARCH: "arm64"},
				)
				return err
			},
		},
		{
			name: "embeds",
			run: func(root, cache, executable string) error {
				_, err := DiscoverGoEmbedFilesV3WithExecutable(
					root, cache, executable,
					GoBuildEnvironment{GOOS: "darwin", GOARCH: "arm64"},
				)
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			root := filepath.Join(dir, "workspace")
			cache := filepath.Join(dir, "cache")
			wrapper := filepath.Join(dir, "go-wrapper")
			logPath := filepath.Join(dir, "calls.log")

			if err := os.MkdirAll(root, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(cache, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(
				filepath.Join(root, "go.mod"),
				[]byte("module example.com/routing\n\ngo 1.26.6\n"),
				0600,
			); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(
				filepath.Join(root, "main.go"),
				[]byte("package main\nfunc main() {}\n"),
				0600,
			); err != nil {
				t.Fatal(err)
			}

			script := "#!/bin/sh\n" +
				"printf '%s/%s %s %s %s\\n' " +
				"\"$GOOS\" \"$GOARCH\" \"$GOTOOLCHAIN\" " +
				"\"$GOPROXY\" \"$*\" >> \"$REW_GO_TEST_LOG\"\n" +
				"exec \"$REW_REAL_GO\" \"$@\"\n"

			if err := os.WriteFile(
				wrapper, []byte(script), 0700,
			); err != nil {
				t.Fatal(err)
			}

			t.Setenv("REW_REAL_GO", realGo)
			t.Setenv("REW_GO_TEST_LOG", logPath)

			if err := tt.run(root, cache, wrapper); err != nil {
				t.Fatal(err)
			}

			data, err := os.ReadFile(logPath)
			if err != nil {
				t.Fatal(err)
			}

			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			if len(lines) != 2 {
				t.Fatalf("expected 2 Go calls, got %d:\n%s", len(lines), data)
			}

			for _, expected := range []string{
				"linux/amd64 local off list -e -json ./...",
				"darwin/arm64 local off list -e -json ./...",
			} {
				if !strings.Contains(string(data), expected) {
					t.Fatalf("missing %q:\n%s", expected, data)
				}
			}
		})
	}
}
