package capsule

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestExtendedSeventeenModuleRoundTripV3(t *testing.T) {
	manifest, files, artifacts :=
		makeExtendedSeventeenModuleWorkspaceV3(t)

	if got := len(manifest.GoSelectedModules); got != 17 {
		t.Fatalf("selected modules = %d, want 17", got)
	}
	if got := len(manifest.GoModules); got != 5 {
		t.Fatalf("full modules = %d, want 5", got)
	}
	if got := len(manifest.GoGraphInfoFiles); got != 12 {
		t.Fatalf("metadata-only modules = %d, want 12", got)
	}
	if got := len(manifest.GoGraphModFiles); got != 12 {
		t.Fatalf("graph mod files = %d, want 12", got)
	}

	var archive bytes.Buffer
	if err := WriteWorkspaceV3(
		&archive, manifest, files, artifacts,
	); err != nil {
		t.Fatalf("write extended capsule: %v", err)
	}

	filename := filepath.Join(t.TempDir(), "extended-17.bfc")
	if err := os.WriteFile(filename, archive.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}

	verified, err := ReadVerifiedV3(filename)
	if err != nil {
		t.Fatalf("verify extended capsule: %v", err)
	}

	if got := len(verified.Manifest.GoSelectedModules); got != 17 {
		t.Fatalf("verified selected modules = %d, want 17", got)
	}
	if got := len(verified.Manifest.GoModules); got != 5 {
		t.Fatalf("verified full modules = %d, want 5", got)
	}
	if got := len(verified.Manifest.GoGraphInfoFiles); got != 12 {
		t.Fatalf("verified metadata-only modules = %d, want 12", got)
	}

	workspaceTAR, err := BuildWorkspaceSnapshotV3(verified)
	if err != nil {
		t.Fatalf("workspace snapshot: %v", err)
	}
	if len(workspaceTAR) == 0 {
		t.Fatal("empty workspace snapshot")
	}

	moduleTAR, err := BuildGoModuleSnapshotV3(verified)
	if err != nil {
		t.Fatalf("module snapshot: %v", err)
	}
	if len(moduleTAR) == 0 {
		t.Fatal("empty module snapshot")
	}

	t.Logf(
		"EXTENDED 17-MODULE ROUNDTRIP: PASS (capsule=%d bytes, workspace=%d bytes, module=%d bytes)",
		archive.Len(),
		len(workspaceTAR),
		len(moduleTAR),
	)
}
