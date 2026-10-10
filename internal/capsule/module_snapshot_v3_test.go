package capsule

import (
	"archive/tar"
	"bytes"
	"io"
	"path"
	"strings"
	"testing"

	"golang.org/x/mod/module"
)

func TestBuildGoModuleSnapshotV3Contents(t *testing.T) {
	verified := makeVerifiedSnapshotFixture(t)

	snapshot, err := BuildGoModuleSnapshotV3(verified)
	if err != nil {
		t.Fatalf("build module snapshot: %v", err)
	}

	mod := verified.Manifest.GoModules[0]

	escapedPath, err := module.EscapePath(mod.Path)
	if err != nil {
		t.Fatal(err)
	}
	escapedVersion, err := module.EscapeVersion(mod.Version)
	if err != nil {
		t.Fatal(err)
	}

	directory := path.Join(
		"cache", "download", escapedPath, "@v",
	)

	reader := tar.NewReader(bytes.NewReader(snapshot))

	for _, ext := range []string{".zip", ".mod", ".info"} {
		header, err := reader.Next()
		if err != nil {
			t.Fatalf("read %s: %v", ext, err)
		}

		expectedPath := path.Join(directory, escapedVersion+ext)
		if header.Name != expectedPath {
			t.Fatalf("path = %q, want %q", header.Name, expectedPath)
		}

		if header.Typeflag != tar.TypeReg {
			t.Fatalf("non-regular TAR entry: %q", header.Name)
		}

		if header.Mode != 0644 {
			t.Fatalf("mode = %o, want 0644", header.Mode)
		}

		content, err := io.ReadAll(reader)
		if err != nil {
			t.Fatal(err)
		}

		want := verified.Artifacts["module-0001"+ext]
		if !bytes.Equal(content, want) {
			t.Fatalf("artifact content mismatch: %s", ext)
		}
	}

	if _, err := reader.Next(); err != io.EOF {
		t.Fatalf("unexpected extra TAR entry: %v", err)
	}
}

func TestBuildGoModuleSnapshotV3RejectsMutations(t *testing.T) {
	for _, ext := range []string{".zip", ".mod", ".info"} {
		t.Run(ext, func(t *testing.T) {
			verified := makeVerifiedSnapshotFixture(t)

			key := "module-0001" + ext
			verified.Artifacts[key][0] ^= 0xff

			if _, err := BuildGoModuleSnapshotV3(verified); err == nil {
				t.Fatalf("accepted modified %s artifact", ext)
			}
		})
	}
}

func TestBuildGoModuleSnapshotV3RejectsMissingArtifact(t *testing.T) {
	verified := makeVerifiedSnapshotFixture(t)

	delete(verified.Artifacts, "module-0001.info")

	if _, err := BuildGoModuleSnapshotV3(verified); err == nil {
		t.Fatal("accepted missing module artifact")
	}
}

func TestBuildGoModuleSnapshotV3RejectsExtraArtifact(t *testing.T) {
	verified := makeVerifiedSnapshotFixture(t)

	verified.Artifacts["unexpected.txt"] = []byte("extra")

	if _, err := BuildGoModuleSnapshotV3(verified); err == nil {
		t.Fatal("accepted unexpected module artifact")
	}
}

func TestBuildGoModuleSnapshotV3RejectsTamperedGoSum(t *testing.T) {
	verified := makeVerifiedSnapshotFixture(t)

	for i := range verified.Files {
		if verified.Files[i].Path == "go.sum" {
			verified.Files[i].Data = []byte("invalid checksum\n")
			break
		}
	}

	if _, err := BuildGoModuleSnapshotV3(verified); err == nil {
		t.Fatal("accepted tampered go.sum")
	}
}

func TestBuildGoModuleSnapshotV3RejectsDuplicateWorkspaceFile(t *testing.T) {
	verified := makeVerifiedSnapshotFixture(t)

	verified.Files[1] = verified.Files[0]

	if _, err := BuildGoModuleSnapshotV3(verified); err == nil {
		t.Fatal("accepted duplicate workspace path")
	}
}

func TestBuildGoModuleSnapshotV3RejectsWrongSchema(t *testing.T) {
	verified := makeVerifiedSnapshotFixture(t)

	verified.Manifest.SchemaVersion = SchemaVersionV2

	if _, err := BuildGoModuleSnapshotV3(verified); err == nil {
		t.Fatal("accepted wrong schema")
	}
}

func TestBuildGoModuleSnapshotV3Deterministic(t *testing.T) {
	verified := makeVerifiedSnapshotFixture(t)

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
		t.Fatal("module snapshot is not deterministic")
	}
}

func TestBuildGoModuleSnapshotV3RejectsInvalidModulePath(t *testing.T) {
	verified := makeVerifiedSnapshotFixture(t)

	verified.Manifest.GoModules[0].Path = "../malicious"

	if _, err := BuildGoModuleSnapshotV3(verified); err == nil {
		t.Fatal("accepted invalid module path")
	}
}

func TestBuildGoModuleSnapshotV3RejectsCorruptChecksum(t *testing.T) {
	verified := makeVerifiedSnapshotFixture(t)

	verified.Manifest.GoModules[0].Artifacts[0].SHA256 =
		"sha256:" + strings.Repeat("0", 64)

	if _, err := BuildGoModuleSnapshotV3(verified); err == nil {
		t.Fatal("accepted incorrect artifact SHA-256")
	}
}

func TestBuildGoModuleSnapshotV3RejectsWrongGoSumChecksum(t *testing.T) {
	verified := makeVerifiedSnapshotFixture(t)

	// Make go.sum internally consistent with its workspace metadata,
	// while keeping an incorrect Go module checksum.
	for i := range verified.Files {
		if verified.Files[i].Path != "go.sum" {
			continue
		}

		original := verified.Files[i].Data
		lines := strings.Split(string(original), "\n")

		changed := false
		for j, line := range lines {
			fields := strings.Fields(line)
			if len(fields) != 3 {
				continue
			}

			fields[2] = "h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
			lines[j] = strings.Join(fields, " ")
			changed = true
			break
		}

		if !changed {
			t.Fatal("fixture has no go.sum checksum")
		}

		verified.Files[i].Data = []byte(strings.Join(lines, "\n"))

		// Update the corresponding workspace metadata so that
		// workspace verification passes.
		for j := range verified.Manifest.Workspace.Files {
			metadata := &verified.Manifest.Workspace.Files[j]
			if metadata.Path == "go.sum" {
				metadata.Size = int64(len(verified.Files[i].Data))
				metadata.SHA256 = SHA256(verified.Files[i].Data)
				break
			}
		}

		break
	}

	if _, err := BuildGoModuleSnapshotV3(verified); err == nil {
		t.Fatal("accepted incorrect Go module checksum")
	}
}
