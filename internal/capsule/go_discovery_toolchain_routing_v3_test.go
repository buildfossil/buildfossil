package capsule

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoDiscoveryUsesSelectedExecutableV3(t *testing.T) {
	tests := []struct {
		name string
		run  func(root, cache, executable string) error
	}{
		{
			name: "source discovery",
			run: func(root, cache, executable string) error {
				_, err := DiscoverGoSourceFilesV3WithExecutable(
					root, cache, executable,
					GoBuildEnvironment{GOOS: "darwin", GOARCH: "arm64"},
				)
				return err
			},
		},
		{
			name: "embed discovery",
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
			executable := filepath.Join(dir, "fake-go")
			logPath := filepath.Join(dir, "invocation.txt")

			if err := os.MkdirAll(root, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(cache, 0700); err != nil {
				t.Fatal(err)
			}

			script := "#!/bin/sh\n" +
				"printf 'ARGS=%s\\n' \"$*\" > \"$REW_GO_TEST_LOG\"\n" +
				"printf 'GOOS=%s\\n' \"$GOOS\" >> \"$REW_GO_TEST_LOG\"\n" +
				"printf 'GOARCH=%s\\n' \"$GOARCH\" >> \"$REW_GO_TEST_LOG\"\n" +
				"printf 'GOTOOLCHAIN=%s\\n' \"$GOTOOLCHAIN\" >> \"$REW_GO_TEST_LOG\"\n" +
				"printf 'GOPROXY=%s\\n' \"$GOPROXY\" >> \"$REW_GO_TEST_LOG\"\n" +
				"printf 'GOSUMDB=%s\\n' \"$GOSUMDB\" >> \"$REW_GO_TEST_LOG\"\n" +
				"printf 'GOWORK=%s\\n' \"$GOWORK\" >> \"$REW_GO_TEST_LOG\"\n" +
				"printf 'GOFLAGS=%s\\n' \"$GOFLAGS\" >> \"$REW_GO_TEST_LOG\"\n" +
				"exit 17\n"

			if err := os.WriteFile(
				executable, []byte(script), 0700,
			); err != nil {
				t.Fatal(err)
			}

			t.Setenv("REW_GO_TEST_LOG", logPath)
			t.Setenv("GOTOOLCHAIN", "auto")

			err := tt.run(root, cache, executable)
			if err == nil {
				t.Fatal("expected fake executable to fail")
			}

			data, readErr := os.ReadFile(logPath)
			if readErr != nil {
				t.Fatalf(
					"selected executable was not invoked: %v",
					readErr,
				)
			}

			got := string(data)
			for _, expected := range []string{
				"ARGS=list -e -json ./...",
				"GOOS=linux",
				"GOARCH=amd64",
				"GOTOOLCHAIN=local",
				"GOPROXY=off",
				"GOSUMDB=off",
				"GOWORK=off",
				"GOFLAGS=-mod=readonly",
			} {
				if !strings.Contains(got, expected) {
					t.Errorf(
						"missing %q in invocation:\n%s",
						expected, got,
					)
				}
			}
		})
	}
}
