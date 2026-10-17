package capsule

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateGoWorkspaceV3WithExecutable(t *testing.T) {
	dir := t.TempDir()
	executable := filepath.Join(dir, "fake-go")

	script := "#!/bin/sh\n" +
		"if [ \"$GOTOOLCHAIN\" != local ]; then exit 9; fi\n" +
		"printf 'off\\n'\n"

	if err := os.WriteFile(executable, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}

	t.Setenv("GOTOOLCHAIN", "auto")

	if err := ValidateGoWorkspaceV3WithExecutable(
		dir, executable,
	); err != nil {
		t.Fatal(err)
	}
}

func TestValidateGoWorkspaceV3WithExecutableRejectsActiveWorkspace(t *testing.T) {
	dir := t.TempDir()
	executable := filepath.Join(dir, "fake-go")

	script := "#!/bin/sh\nprintf '/tmp/project/go.work\\n'\n"

	if err := os.WriteFile(executable, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}

	err := ValidateGoWorkspaceV3WithExecutable(dir, executable)
	if err == nil || !strings.Contains(err.Error(), "active go.work") {
		t.Fatalf("expected active workspace rejection, got %v", err)
	}
}

func TestValidateGoWorkspaceV3WithExecutableRejectsRelativePath(t *testing.T) {
	if err := ValidateGoWorkspaceV3WithExecutable(
		t.TempDir(), "go",
	); err == nil {
		t.Fatal("expected relative executable rejection")
	}
}
