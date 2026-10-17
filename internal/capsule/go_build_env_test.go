package capsule

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidatePortableGoBuild(t *testing.T) {
	tests := []struct {
		name    string
		env     GoBuildEnvironment
		wantErr string
	}{
		{
			name: "supported darwin arm64",
			env: GoBuildEnvironment{
				GOOS:       "darwin",
				GOARCH:     "arm64",
				CGOEnabled: "0",
				GOFLAGS:    "",
			},
		},
		{
			name: "supported linux amd64",
			env: GoBuildEnvironment{
				GOOS:       "linux",
				GOARCH:     "amd64",
				CGOEnabled: "0",
				GOFLAGS:    "",
			},
		},
		{
			name: "CGO enabled",
			env: GoBuildEnvironment{
				GOOS:       "darwin",
				GOARCH:     "arm64",
				CGOEnabled: "1",
			},
			wantErr: "CGO_ENABLED=0",
		},
		{
			name: "custom GOFLAGS",
			env: GoBuildEnvironment{
				GOOS:       "darwin",
				GOARCH:     "arm64",
				CGOEnabled: "0",
				GOFLAGS:    "-tags=custom",
			},
			wantErr: "empty GOFLAGS",
		},
		{
			name: "unsupported platform",
			env: GoBuildEnvironment{
				GOOS:       "windows",
				GOARCH:     "amd64",
				CGOEnabled: "0",
			},
			wantErr: "unsupported portable Go source platform",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.env.ValidatePortableGoBuild()

			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}

			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestInspectGoBuildEnvironment(t *testing.T) {
	t.Setenv("GOOS", "darwin")
	t.Setenv("GOARCH", "arm64")
	t.Setenv("CGO_ENABLED", "0")
	t.Setenv("GOFLAGS", "")

	env, err := InspectGoBuildEnvironment(t.TempDir())
	if err != nil {
		t.Fatalf("inspect environment: %v", err)
	}

	if err := env.ValidatePortableGoBuild(); err != nil {
		t.Fatalf("expected supported environment: %v", err)
	}

	if env.GOOS != "darwin" ||
		env.GOARCH != "arm64" ||
		env.CGOEnabled != "0" ||
		env.GOFLAGS != "" {
		t.Fatalf("unexpected environment: %+v", env)
	}
}

func TestInspectGoBuildEnvironmentWithExecutable(t *testing.T) {
	dir := t.TempDir()
	goExecutable := filepath.Join(dir, "fake-go")

	script := "#!/bin/sh\n" +
		"if [ \"$GOTOOLCHAIN\" != local ]; then exit 9; fi\n" +
		"printf 'darwin\\narm64\\n0\\n\\n'\n"

	if err := os.WriteFile(
		goExecutable, []byte(script), 0700,
	); err != nil {
		t.Fatal(err)
	}

	t.Setenv("GOTOOLCHAIN", "auto")

	env, err := InspectGoBuildEnvironmentWithExecutable(
		dir, goExecutable,
	)
	if err != nil {
		t.Fatal(err)
	}

	if env.GOOS != "darwin" ||
		env.GOARCH != "arm64" ||
		env.CGOEnabled != "0" ||
		env.GOFLAGS != "" {
		t.Fatalf("unexpected Go environment: %+v", env)
	}
}

func TestInspectGoBuildEnvironmentWithRelativeExecutable(t *testing.T) {
	_, err := InspectGoBuildEnvironmentWithExecutable(
		t.TempDir(), "go",
	)
	if err == nil {
		t.Fatal("expected relative executable rejection")
	}
}
