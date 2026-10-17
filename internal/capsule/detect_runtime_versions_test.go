package capsule

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectRuntimeWithExecutableExactVersions(t *testing.T) {
	tests := []struct {
		version string
		wantOK  bool
	}{
		{"1.26.6", true},
		{"1.25.14", true},
		{"1.25.13", false},
		{"1.26.5", false},
		{"1.27.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			dir := t.TempDir()
			executable := filepath.Join(dir, "fake-go")

			script := "#!/bin/sh\n" +
				"if [ \"$GOTOOLCHAIN\" != local ]; then exit 9; fi\n" +
				"printf 'go version go" + tt.version + " darwin/arm64\\n'\n"

			if err := os.WriteFile(executable, []byte(script), 0700); err != nil {
				t.Fatal(err)
			}

			t.Setenv("GOTOOLCHAIN", "auto")

			got := DetectRuntimeWithExecutable(
				[]string{"go", "build", "./..."},
				dir,
				executable,
			)

			if tt.wantOK {
				if got == nil {
					t.Fatalf("supported Go %s rejected", tt.version)
				}
				if got.Version != tt.version {
					t.Fatalf("version = %q, want %q", got.Version, tt.version)
				}
			} else if got != nil {
				t.Fatalf("unsupported Go %s accepted: %+v", tt.version, got)
			}
		})
	}
}
