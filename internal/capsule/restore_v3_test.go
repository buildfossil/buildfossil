package capsule

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func makeVerifiedCapsuleV3(t *testing.T) VerifiedCapsuleV3 {
	t.Helper()

	manifest, files, artifacts := makeWorkspaceV3Fixture(t)

	var archive bytes.Buffer
	if err := WriteWorkspaceV3(&archive, manifest, files, artifacts); err != nil {
		t.Fatal(err)
	}

	capsulePath := filepath.Join(t.TempDir(), "failure.bfc")
	if err := os.WriteFile(capsulePath, archive.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}

	verified, err := ReadVerifiedV3(capsulePath)
	if err != nil {
		t.Fatal(err)
	}

	return verified
}

func TestRestoreWorkspaceV3RoundTrip(t *testing.T) {
	verified := makeVerifiedCapsuleV3(t)

	workspaceRoot := t.TempDir()
	artifactsRoot := t.TempDir()

	if err := RestoreWorkspaceV3(
		workspaceRoot,
		artifactsRoot,
		verified,
	); err != nil {
		t.Fatalf("restore v3 capsule: %v", err)
	}

	for _, expected := range verified.Files {
		actual, err := os.ReadFile(
			filepath.Join(workspaceRoot, filepath.FromSlash(expected.Path)),
		)
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(actual, expected.Data) {
			t.Fatalf("workspace content mismatch: %s", expected.Path)
		}
	}

	version := verified.Manifest.GoModules[0].Version

	for _, artifact := range verified.Manifest.GoModules[0].Artifacts {
		ext := artifact.Path[len("module-0001"):]
		name := version + ext

		actual, err := os.ReadFile(filepath.Join(artifactsRoot, name))
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(actual, verified.Artifacts[artifact.Path]) {
			t.Fatalf("artifact content mismatch: %s", name)
		}
	}
}

func TestRestoreWorkspaceV3RejectsCorruptedArtifacts(t *testing.T) {
	verified := makeVerifiedCapsuleV3(t)

	verified.Artifacts["module-0001.zip"][0] ^= 0xff

	workspaceRoot := t.TempDir()
	artifactsRoot := t.TempDir()

	if err := RestoreWorkspaceV3(
		workspaceRoot,
		artifactsRoot,
		verified,
	); err == nil {
		t.Fatal("corrupted artifacts accepted")
	}

	for _, root := range []string{workspaceRoot, artifactsRoot} {
		entries, err := os.ReadDir(root)
		if err != nil {
			t.Fatal(err)
		}

		if len(entries) != 0 {
			t.Fatalf("restore wrote files despite verification failure: %s", root)
		}
	}
}

func TestRestoreWorkspaceV3RejectsOverlappingRoots(t *testing.T) {
	verified := makeVerifiedCapsuleV3(t)

	parent := t.TempDir()
	child := filepath.Join(parent, "nested")

	if err := os.Mkdir(child, 0700); err != nil {
		t.Fatal(err)
	}

	for _, roots := range [][2]string{
		{parent, parent},
		{parent, child},
		{child, parent},
	} {
		if err := RestoreWorkspaceV3(
			roots[0],
			roots[1],
			verified,
		); err == nil {
			t.Fatalf("overlapping roots accepted: %v", roots)
		}
	}
}

func TestRestoreWorkspaceV3RejectsNonemptyDestination(t *testing.T) {
	verified := makeVerifiedCapsuleV3(t)

	workspaceRoot := t.TempDir()
	artifactsRoot := t.TempDir()

	original := []byte("must remain untouched")

	path := filepath.Join(artifactsRoot, "existing.txt")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}

	if err := RestoreWorkspaceV3(
		workspaceRoot,
		artifactsRoot,
		verified,
	); err == nil {
		t.Fatal("nonempty destination accepted")
	}

	actual, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(actual, original) {
		t.Fatal("existing file was modified")
	}

	entries, err := os.ReadDir(workspaceRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatal("workspace was modified despite invalid destination")
	}
}

func TestRestoreWorkspaceV3RejectsAliasedRoots(t *testing.T) {
	verified := makeVerifiedCapsuleV3(t)

	parent := t.TempDir()
	workspaceRoot := filepath.Join(parent, "workspace")

	if err := os.Mkdir(workspaceRoot, 0700); err != nil {
		t.Fatal(err)
	}

	aliasParent := t.TempDir()
	alias := filepath.Join(aliasParent, "alias")

	if err := os.Symlink(parent, alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	artifactsRoot := filepath.Join(alias, "workspace")

	if err := RestoreWorkspaceV3(
		workspaceRoot,
		artifactsRoot,
		verified,
	); err == nil {
		t.Fatal("aliased restore roots accepted")
	}

	entries, err := os.ReadDir(workspaceRoot)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 0 {
		t.Fatal("restore modified aliased directory")
	}
}
