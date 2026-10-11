package capsule

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

type multiModuleTarEntryV3 struct {
	header tar.Header
	data   []byte
}

func makeMultiModuleSecurityArchiveV3(t *testing.T) []byte {
	t.Helper()

	a, aFiles, aSum := makeIndexedGoModuleFixture(
		t, 1, "example.com/a",
	)
	b, bFiles, bSum := makeIndexedGoModuleFixture(
		t, 2, "example.com/b",
	)

	artifacts := make(map[string][]byte)
	for name, data := range aFiles {
		artifacts[name] = data
	}
	for name, data := range bFiles {
		artifacts[name] = data
	}

	files := []WorkspaceFile{
		{
			Path: "go.mod",
			Data: []byte(`module example.com/app

go 1.26.6

require (
	example.com/a v1.0.0
	example.com/b v1.0.0
)
`),
			Mode: 0644,
		},
		{
			Path: "go.sum",
			Data: append(append([]byte(nil), aSum...), bSum...),
			Mode: 0644,
		},
		{
			Path: "main.go",
			Data: []byte("package main\nfunc main() { missing() }\n"),
			Mode: 0644,
		},
	}

	manifest := Manifest{
		SchemaVersion: SchemaVersionV3,
		Runtime: &Runtime{
			Kind: "go", Version: SupportedGoVersion,
		},
		GoBuildEnv: &GoBuildEnvironment{
			GOOS: "linux", GOARCH: "amd64", CGOEnabled: "0",
		},
		GoModules: []GoModuleV3{a, b},
		Execution: Execution{
			Argv:       []string{"go", "build", "./..."},
			WorkingDir: ".",
			ExitCode:   1,
			Stderr:     "missing\n",
		},
		Platform: Platform{
			OS: "darwin", Architecture: "arm64",
		},
	}

	var archive bytes.Buffer
	if err := WriteWorkspaceV3(
		&archive, manifest, files, artifacts,
	); err != nil {
		t.Fatal(err)
	}

	return archive.Bytes()
}

func TestMultiModuleV3ReaderRejectsCorruptTAR(t *testing.T) {
	tests := []struct {
		name   string
		mutate func([]multiModuleTarEntryV3) []multiModuleTarEntryV3
	}{
		{
			name: "missing second module ZIP",
			mutate: func(entries []multiModuleTarEntryV3) []multiModuleTarEntryV3 {
				for i, e := range entries {
					if e.header.Name == "dependencies/module-0002.zip" {
						return append(entries[:i], entries[i+1:]...)
					}
				}
				return entries
			},
		},
		{
			name: "reordered second module artifacts",
			mutate: func(entries []multiModuleTarEntryV3) []multiModuleTarEntryV3 {
				zipIndex, modIndex := -1, -1
				for i, e := range entries {
					switch e.header.Name {
					case "dependencies/module-0002.zip":
						zipIndex = i
					case "dependencies/module-0002.mod":
						modIndex = i
					}
				}
				if zipIndex >= 0 && modIndex >= 0 {
					entries[zipIndex], entries[modIndex] =
						entries[modIndex], entries[zipIndex]
				}
				return entries
			},
		},
		{
			name: "tampered second module ZIP",
			mutate: func(entries []multiModuleTarEntryV3) []multiModuleTarEntryV3 {
				for i := range entries {
					if entries[i].header.Name == "dependencies/module-0002.zip" {
						entries[i].data[0] ^= 0xff
					}
				}
				return entries
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			original := makeMultiModuleSecurityArchiveV3(t)
			tr := tar.NewReader(bytes.NewReader(original))

			var entries []multiModuleTarEntryV3
			for {
				header, err := tr.Next()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatal(err)
				}

				data, err := io.ReadAll(tr)
				if err != nil {
					t.Fatal(err)
				}

				entries = append(entries, multiModuleTarEntryV3{
					header: *header,
					data:   data,
				})
			}

			entries = tc.mutate(entries)

			var modified bytes.Buffer
			tw := tar.NewWriter(&modified)

			for _, entry := range entries {
				entry.header.Size = int64(len(entry.data))

				if err := tw.WriteHeader(&entry.header); err != nil {
					t.Fatal(err)
				}
				if _, err := tw.Write(entry.data); err != nil {
					t.Fatal(err)
				}
			}

			if err := tw.Close(); err != nil {
				t.Fatal(err)
			}

			path := filepath.Join(t.TempDir(), "corrupt.bfc")
			if err := os.WriteFile(path, modified.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}

			if _, err := ReadVerifiedV3(path); err == nil {
				t.Fatal("reader accepted corrupted multi-module TAR")
			}
		})
	}
}
