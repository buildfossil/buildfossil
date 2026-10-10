package capsule

import (
	"archive/tar"
	"bytes"
	"io"
	"testing"
)

func makeWorkspaceV3Fixture(t *testing.T) (
	Manifest,
	[]WorkspaceFile,
	map[string][]byte,
) {
	t.Helper()

	mod, artifacts, goSum := makeGoModuleFixture(t)

	manifest := validManifestV3()
	manifest.GoModules = []GoModuleV3{mod}

	files := []WorkspaceFile{
		{
			Path: "go.mod",
			Data: []byte(
				"module example.com/test\n\ngo 1.26.0\n" +
					"require " + mod.Path + " " + mod.Version + "\n",
			),
			Mode: 0600,
		},
		{
			Path: "go.sum",
			Data: goSum,
			Mode: 0600,
		},
		{
			Path: "main.go",
			Data: []byte(
				"package main\n\nimport \"" + mod.Path + "\"\n" +
					"func main() { _ = fixture.Value() }\n",
			),
			Mode: 0600,
		},
	}

	return manifest, files, artifacts
}

func TestWriteWorkspaceV3ArchiveEntries(t *testing.T) {
	manifest, files, artifacts := makeWorkspaceV3Fixture(t)

	var output bytes.Buffer

	if err := WriteWorkspaceV3(
		&output,
		manifest,
		files,
		artifacts,
	); err != nil {
		t.Fatalf("write v3 capsule: %v", err)
	}

	if int64(output.Len()) > MaxCapsuleSize {
		t.Fatalf("capsule too large: %d", output.Len())
	}

	tr := tar.NewReader(bytes.NewReader(output.Bytes()))

	expectedNames := []string{
		"manifest.json",
		"workspace/go.mod",
		"workspace/go.sum",
		"workspace/main.go",
		"dependencies/module-0001.zip",
		"dependencies/module-0001.mod",
		"dependencies/module-0001.info",
	}

	for _, expected := range expectedNames {
		header, err := tr.Next()
		if err != nil {
			t.Fatalf("read %q: %v", expected, err)
		}

		if header.Name != expected {
			t.Fatalf(
				"entry = %q; want %q",
				header.Name,
				expected,
			)
		}

		if header.Typeflag != tar.TypeReg {
			t.Fatalf("non-regular TAR entry: %q", header.Name)
		}

		if _, err := io.Copy(io.Discard, tr); err != nil {
			t.Fatalf("read TAR entry %q: %v", header.Name, err)
		}
	}

	if _, err := tr.Next(); err != io.EOF {
		t.Fatalf("unexpected extra TAR entry: %v", err)
	}
}

func TestWriteWorkspaceV3RejectsInvalidArtifacts(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Manifest, *[]WorkspaceFile, map[string][]byte)
	}{
		{
			name: "missing go.sum",
			mutate: func(_ *Manifest, files *[]WorkspaceFile, _ map[string][]byte) {
				*files = append((*files)[:1], (*files)[2:]...)
			},
		},
		{
			name: "modified ZIP",
			mutate: func(_ *Manifest, _ *[]WorkspaceFile, artifacts map[string][]byte) {
				artifacts["module-0001.zip"][0] ^= 0xff
			},
		},
		{
			name: "missing artifact",
			mutate: func(_ *Manifest, _ *[]WorkspaceFile, artifacts map[string][]byte) {
				delete(artifacts, "module-0001.info")
			},
		},
		{
			name: "wrong schema version",
			mutate: func(m *Manifest, _ *[]WorkspaceFile, _ map[string][]byte) {
				m.SchemaVersion = SchemaVersionV2
			},
		},
		{
			name: "no modules",
			mutate: func(m *Manifest, _ *[]WorkspaceFile, _ map[string][]byte) {
				m.GoModules = nil
			},
		},
		{
			name: "duplicate workspace path",
			mutate: func(_ *Manifest, files *[]WorkspaceFile, _ map[string][]byte) {
				*files = append(*files, (*files)[0])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest, files, artifacts := makeWorkspaceV3Fixture(t)
			tt.mutate(&manifest, &files, artifacts)

			var output bytes.Buffer

			err := WriteWorkspaceV3(&output, manifest, files, artifacts)
			if err == nil {
				t.Fatal("invalid v3 capsule accepted")
			}

			if output.Len() != 0 {
				t.Fatalf(
					"writer emitted %d bytes despite validation failure",
					output.Len(),
				)
			}
		})
	}
}
