package capsule

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoModuleCommandsUseSelectedExecutableV3(t *testing.T) {
	tests := []struct {
		name string
		want string
		run  func(workspace, cache, executable string) error
	}{
		{
			name: "module graph",
			want: "list -m -json all",
			run: func(workspace, cache, executable string) error {
				_, err := ListGoModuleGraphV3WithExecutable(
					workspace, cache, executable,
				)
				return err
			},
		},
		{
			name: "build modules",
			want: "list -deps -e -json ./...",
			run: func(workspace, cache, executable string) error {
				_, err := ListGoBuildModulesV3WithExecutable(
					workspace, cache, nil, executable,
				)
				return err
			},
		},
		{
			name: "graph requirements",
			want: "mod graph",
			run: func(workspace, cache, executable string) error {
				_, err := ListGoGraphModRequirementsV3WithExecutable(
					workspace, cache, "example.com/test", nil, executable,
				)
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			executable := filepath.Join(dir, "fake-go")
			logPath := filepath.Join(dir, "invocation.txt")
			cache := filepath.Join(dir, "cache")
			workspace := filepath.Join(dir, "workspace")

			if err := os.MkdirAll(workspace, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(cache, 0700); err != nil {
				t.Fatal(err)
			}

			script := "#!/bin/sh\n" +
				"printf '%s\\n' \"$*\" > \"$REW_GO_TEST_LOG\"\n" +
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

			err := tt.run(workspace, cache, executable)
			if err == nil {
				t.Fatal("expected fake Go executable to fail")
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
				tt.want,
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
