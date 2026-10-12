package capsule

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceV3LongUSTARPathRoundTrip(t *testing.T) {
	const longPath = "internal/toml-test/tests/invalid/string/" +
		"basic-multiline-out-of-range-unicode-escape-01.toml"

	manifest, files := largeWorkspaceV3Fixture()

	// Keep the fixture small: this test targets TAR path encoding,
	// not workspace capacity.
	files = append([]WorkspaceFile(nil), files[:3]...)
	files[2].Path = longPath
	files[2].Data = []byte("real-world USTAR path fixture\n")

	var archive bytes.Buffer
	if err := WriteWorkspaceV3(&archive, manifest, files, nil); err != nil {
		t.Fatalf("write capsule: %v", err)
	}

	// The canonical capsule must contain only the expected TAR entries.
	reader := tar.NewReader(bytes.NewReader(archive.Bytes()))
	found := false

	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if header.Name == "workspace/"+longPath {
			found = true
			if header.Format != tar.FormatUSTAR {
				t.Fatalf("long path format = %v, want USTAR", header.Format)
			}
		}
		if _, err := io.Copy(io.Discard, reader); err != nil {
			t.Fatal(err)
		}
	}

	if !found {
		t.Fatal("long USTAR path missing from capsule")
	}

	path := filepath.Join(t.TempDir(), "ustar.bfc")
	if err := os.WriteFile(path, archive.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}

	verified, err := ReadVerifiedV3(path)
	if err != nil {
		t.Fatalf("verify capsule: %v", err)
	}

	found = false
	for _, file := range verified.Files {
		if file.Path == longPath {
			found = true
			if !bytes.Equal(file.Data, files[2].Data) {
				t.Fatal("verified file contents differ")
			}
		}
	}
	if !found {
		t.Fatal("long path missing after verification")
	}

	snapshot, err := BuildWorkspaceSnapshotV3(verified)
	if err != nil {
		t.Fatalf("build Docker workspace snapshot: %v", err)
	}

	snapshotReader := tar.NewReader(bytes.NewReader(snapshot))
	found = false
	for {
		header, err := snapshotReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if header.Name == longPath {
			found = true
		}
		if _, err := io.Copy(io.Discard, snapshotReader); err != nil {
			t.Fatal(err)
		}
	}
	if !found {
		t.Fatal("long path missing from Docker snapshot")
	}

	// A long, unsplittable name must not silently introduce PAX.
	invalid := strings.Repeat("x", 120)
	files[2].Path = invalid

	var rejected bytes.Buffer
	if err := WriteWorkspaceV3(&rejected, manifest, files, nil); err == nil {
		t.Fatal("expected non-USTAR path rejection")
	}
	if rejected.Len() != 0 {
		t.Fatal("invalid path should be rejected before archive output")
	}
}
