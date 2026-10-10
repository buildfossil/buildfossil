package capsule

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateGoWorkspaceV3(t *testing.T) {
	root := t.TempDir()

	goMod := []byte("module example.com/workspace-test\n\ngo 1.26.6\n")
	if err := os.WriteFile(
		filepath.Join(root, "go.mod"),
		goMod,
		0600,
	); err != nil {
		t.Fatal(err)
	}

	t.Run("module mode accepted", func(t *testing.T) {
		t.Setenv("GOWORK", "off")

		if err := ValidateGoWorkspaceV3(root); err != nil {
			t.Fatalf("module mode rejected: %v", err)
		}
	})

	t.Run("active workspace rejected", func(t *testing.T) {
		workFile := "go 1.26.6\n\nuse .\n"

		if err := os.WriteFile(
			filepath.Join(root, "go.work"),
			[]byte(workFile),
			0600,
		); err != nil {
			t.Fatal(err)
		}

		t.Setenv("GOWORK", "auto")

		err := ValidateGoWorkspaceV3(root)
		if err == nil {
			t.Fatal("active go.work was accepted")
		}
		if !strings.Contains(err.Error(), "active go.work") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("explicit workspace disabled", func(t *testing.T) {
		t.Setenv("GOWORK", "off")

		if err := ValidateGoWorkspaceV3(root); err != nil {
			t.Fatalf("GOWORK=off rejected: %v", err)
		}
	})
}
