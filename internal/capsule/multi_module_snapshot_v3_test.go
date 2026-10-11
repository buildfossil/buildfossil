package capsule

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"testing"

	"golang.org/x/mod/module"
)

func makeVerifiedMultiModuleSnapshotV3(
	t *testing.T,
) VerifiedCapsuleV3 {
	t.Helper()

	archive := makeMultiModuleSecurityArchiveV3(t)
	filename := filepath.Join(t.TempDir(), "multi.bfc")

	if err := os.WriteFile(filename, archive, 0600); err != nil {
		t.Fatal(err)
	}

	verified, err := ReadVerifiedV3(filename)
	if err != nil {
		t.Fatal(err)
	}

	return verified
}

func TestBuildGoModuleSnapshotV3TwoModules(t *testing.T) {
	verified := makeVerifiedMultiModuleSnapshotV3(t)

	snapshot, err := BuildGoModuleSnapshotV3(verified)
	if err != nil {
		t.Fatalf("build multi-module snapshot: %v", err)
	}

	reader := tar.NewReader(bytes.NewReader(snapshot))

	for i, mod := range verified.Manifest.GoModules {
		escapedPath, err := module.EscapePath(mod.Path)
		if err != nil {
			t.Fatal(err)
		}

		escapedVersion, err := module.EscapeVersion(mod.Version)
		if err != nil {
			t.Fatal(err)
		}

		for _, ext := range []string{".zip", ".mod", ".info"} {
			header, err := reader.Next()
			if err != nil {
				t.Fatalf("module %d %s: %v", i+1, ext, err)
			}

			expectedPath := path.Join(
				"cache",
				"download",
				escapedPath,
				"@v",
				escapedVersion+ext,
			)

			if header.Name != expectedPath {
				t.Fatalf(
					"module %d path = %q, want %q",
					i+1, header.Name, expectedPath,
				)
			}

			if header.Typeflag != tar.TypeReg ||
				header.Mode != 0644 {
				t.Fatalf("invalid TAR metadata: %s", header.Name)
			}

			data, err := io.ReadAll(reader)
			if err != nil {
				t.Fatal(err)
			}

			key := fmt.Sprintf("module-%04d%s", i+1, ext)
			if !bytes.Equal(data, verified.Artifacts[key]) {
				t.Fatalf("artifact mismatch: %s", key)
			}
		}
	}

	if _, err := reader.Next(); err != io.EOF {
		t.Fatalf("unexpected extra TAR entry: %v", err)
	}
}

func TestBuildGoModuleSnapshotV3TwoModulesDeterministic(t *testing.T) {
	verified := makeVerifiedMultiModuleSnapshotV3(t)

	first, err := BuildGoModuleSnapshotV3(verified)
	if err != nil {
		t.Fatal(err)
	}

	for i, j := 0, len(verified.Files)-1; i < j; i, j = i+1, j-1 {
		verified.Files[i], verified.Files[j] =
			verified.Files[j], verified.Files[i]
	}

	second, err := BuildGoModuleSnapshotV3(verified)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(first, second) {
		t.Fatal("multi-module snapshot is not deterministic")
	}
}

func TestBuildGoModuleSnapshotV3RejectsTamperedSecondModule(t *testing.T) {
	verified := makeVerifiedMultiModuleSnapshotV3(t)

	verified.Artifacts["module-0002.zip"][0] ^= 0xff

	if _, err := BuildGoModuleSnapshotV3(verified); err == nil {
		t.Fatal("snapshot accepted tampered second module")
	}
}
