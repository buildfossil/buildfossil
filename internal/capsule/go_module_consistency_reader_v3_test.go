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

func TestReadVerifiedV3RejectsInconsistentGoMod(t *testing.T) {
	original := zeroDependencyFixtureV3(t)

	reader := tar.NewReader(bytes.NewReader(original))

	type entry struct {
		header tar.Header
		data   []byte
	}

	var entries []entry
	var manifest Manifest

	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}

		data, err := io.ReadAll(reader)
		if err != nil {
			t.Fatal(err)
		}

		if header.Name == "manifest.json" {
			if err := json.Unmarshal(data, &manifest); err != nil {
				t.Fatal(err)
			}
		}

		entries = append(entries, entry{
			header: *header,
			data:   data,
		})
	}

	// Inject a dependency into go.mod without declaring it
	// in manifest.GoModules.
	changedGoMod := []byte(
		"module example.com/zero\n\n" +
			"go 1.26.6\n\n" +
			"require example.com/undeclared v1.0.0\n",
	)

	found := false

	for i := range manifest.Workspace.Files {
		if manifest.Workspace.Files[i].Path == "go.mod" {
			manifest.Workspace.Files[i].Size = int64(len(changedGoMod))
			manifest.Workspace.Files[i].SHA256 = SHA256(changedGoMod)
			found = true
		}
	}

	if !found {
		t.Fatal("fixture missing go.mod")
	}

	manifestData, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}

	for i := range entries {
		switch entries[i].header.Name {
		case "manifest.json":
			entries[i].data = manifestData
		case "workspace/go.mod":
			entries[i].data = changedGoMod
		}
	}

	var modified bytes.Buffer
	writer := tar.NewWriter(&modified)

	for _, e := range entries {
		header := e.header
		header.Size = int64(len(e.data))

		if err := writer.WriteHeader(&header); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(e.data); err != nil {
			t.Fatal(err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	filename := filepath.Join(t.TempDir(), "inconsistent.bfc")
	if err := os.WriteFile(filename, modified.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}

	if _, err := ReadVerifiedV3(filename); err == nil {
		t.Fatal("reader accepted go.mod inconsistent with manifest")
	}
}
