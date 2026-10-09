package capsule

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateDemoCapsule(t *testing.T) {
	if os.Getenv("BUILDFOSSIL_GENERATE_DEMO") != "1" {
		t.Skip("demo generation disabled")
	}

	output := filepath.Join(t.TempDir(), "demo.bfc")

	content := []byte("BUILD_FOSSIL_TEST_FAILURE\n")

	manifest := Manifest{
		SchemaVersion: SchemaVersion,
		Execution: Execution{
			Argv: []string{
				"/bin/sh",
				"-c",
				"cat fixture.txt >&2; exit 17",
			},
			WorkingDir: ".",
			ExitCode:   17,
			Stderr:     string(content),
		},
		Platform: Platform{
			OS:           "linux",
			Architecture: "amd64",
		},
	}

	file := WorkspaceFile{
		Path: "fixture.txt",
		Data: content,
		Mode: 0600,
	}

	if err := WriteFile(output, manifest, file); err != nil {
		t.Fatal(err)
	}

	verified, err := ReadVerified(output)
	if err != nil {
		t.Fatal(err)
	}

	if len(verified.Manifest.Workspace.Files) != 1 {
		t.Fatal("missing workspace metadata")
	}

	t.Logf("validated synthetic capsule: %s", output)
}
