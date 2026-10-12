package capsule

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestWorkspaceV3LongUSTARPathStoreRoundTrip(t *testing.T) {
	const longPath = "internal/toml-test/tests/invalid/string/" +
		"basic-multiline-out-of-range-unicode-escape-01.toml"

	manifest, files := largeWorkspaceV3Fixture()
	files = append([]WorkspaceFile(nil), files[:3]...)
	files[2].Path = longPath
	files[2].Data = []byte("USTAR store-based writer regression\n")

	store, err := NewArtifactStoreV3(1 << 20)
	if err != nil {
		t.Fatalf("create artifact store: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Errorf("close artifact store: %v", err)
		}
	}()

	var output bytes.Buffer
	if err := WriteWorkspaceFromStoreV3(
		&output, manifest, files, store,
	); err != nil {
		t.Fatalf("store writer: %v", err)
	}

	filename := filepath.Join(t.TempDir(), "store-ustar.bfc")
	if err := os.WriteFile(filename, output.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}

	verified, err := ReadVerifiedToStoreV3(filename)
	if err != nil {
		t.Fatalf("verified store reader: %v", err)
	}
	defer func() {
		if err := verified.Close(); err != nil {
			t.Errorf("close store: %v", err)
		}
	}()

	if len(verified.Files) != len(files) {
		t.Fatalf("verified %d files, want %d",
			len(verified.Files), len(files))
	}

	found := false
	for _, file := range verified.Files {
		if file.Path == longPath {
			found = true
			if !bytes.Equal(file.Data, files[2].Data) {
				t.Fatal("long path content mismatch")
			}
		}
	}
	if !found {
		t.Fatal("long path missing from verified store")
	}

	tr := tar.NewReader(bytes.NewReader(output.Bytes()))
	entries := 0
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}

		entries++

		if h.Name == "workspace/"+longPath &&
			h.Format != tar.FormatUSTAR {
			t.Fatalf("long path format = %v, want USTAR", h.Format)
		}

		if _, err := io.Copy(io.Discard, tr); err != nil {
			t.Fatal(err)
		}
	}

	if want := 1 + len(files); entries != want {
		t.Fatalf("TAR entries = %d, want %d", entries, want)
	}

	if got := int64(output.Len()); got != expectedTarSize(
		int64(len(mustMarshalManifestV3(t, verified.Manifest))),
		verified.Manifest.Workspace,
	) {
		t.Fatalf("archive size = %d; canonical size mismatch", got)
	}
}

func mustMarshalManifestV3(t *testing.T, m Manifest) []byte {
	t.Helper()
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
