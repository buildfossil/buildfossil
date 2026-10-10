package capsule

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestVerifiedCapsuleV3RoundTrip(t *testing.T) {
	manifest, files, artifacts := makeWorkspaceV3Fixture(t)

	var archive bytes.Buffer

	if err := WriteWorkspaceV3(
		&archive,
		manifest,
		files,
		artifacts,
	); err != nil {
		t.Fatalf("write v3 capsule: %v", err)
	}

	capsulePath := filepath.Join(t.TempDir(), "failure.bfc")

	if err := os.WriteFile(
		capsulePath,
		archive.Bytes(),
		0600,
	); err != nil {
		t.Fatal(err)
	}

	verified, err := ReadVerifiedV3(capsulePath)
	if err != nil {
		t.Fatalf("read verified v3 capsule: %v", err)
	}

	if verified.Manifest.SchemaVersion != SchemaVersionV3 {
		t.Fatalf(
			"schema version = %d; want %d",
			verified.Manifest.SchemaVersion,
			SchemaVersionV3,
		)
	}

	if len(verified.Manifest.GoModules) != 1 {
		t.Fatalf(
			"module count = %d; want 1",
			len(verified.Manifest.GoModules),
		)
	}

	if len(verified.Files) != len(files) {
		t.Fatalf(
			"workspace file count = %d; want %d",
			len(verified.Files),
			len(files),
		)
	}

	for i, expected := range files {
		actual := verified.Files[i]

		if actual.Path != expected.Path ||
			!bytes.Equal(actual.Data, expected.Data) {
			t.Fatalf(
				"workspace file mismatch at index %d: %s",
				i,
				expected.Path,
			)
		}
	}

	for _, name := range goArtifactNamesV3 {
		if !bytes.Equal(verified.Artifacts[name], artifacts[name]) {
			t.Fatalf("Go artifact mismatch: %s", name)
		}
	}
}

func TestVerifiedCapsuleV3RejectsTamperedData(t *testing.T) {
	tests := []struct {
		name   string
		target string
	}{
		{
			name:   "modified workspace",
			target: "workspace/main.go",
		},
		{
			name:   "modified module ZIP",
			target: "dependencies/module-0001.zip",
		},
		{
			name:   "modified module metadata",
			target: "dependencies/module-0001.mod",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest, files, artifacts := makeWorkspaceV3Fixture(t)

			var archive bytes.Buffer
			if err := WriteWorkspaceV3(
				&archive, manifest, files, artifacts,
			); err != nil {
				t.Fatal(err)
			}

			data := append([]byte(nil), archive.Bytes()...)

			tr := tar.NewReader(bytes.NewReader(data))
			var offset int64

			for {
				header, err := tr.Next()
				if err == io.EOF {
					t.Fatalf("target not found: %s", tt.target)
				}
				if err != nil {
					t.Fatal(err)
				}

				if header.Name == tt.target {
					if header.Size == 0 {
						t.Fatal("cannot corrupt empty entry")
					}

					// Locate the content by matching its bytes.
					content := make([]byte, header.Size)
					if _, err := io.ReadFull(tr, content); err != nil {
						t.Fatal(err)
					}

					index := bytes.Index(data[offset:], content)
					if index < 0 {
						t.Fatal("could not locate TAR entry content")
					}

					data[offset+int64(index)] ^= 0xff
					break
				}

				offset += tarEntrySize(header.Size)
			}

			capsulePath := filepath.Join(
				t.TempDir(), "tampered.bfc",
			)
			if err := os.WriteFile(capsulePath, data, 0600); err != nil {
				t.Fatal(err)
			}

			if _, err := ReadVerifiedV3(capsulePath); err == nil {
				t.Fatal("tampered capsule accepted")
			}
		})
	}
}

func TestVerifiedCapsuleV3RejectsInvalidTarStructure(t *testing.T) {
	tests := []struct {
		name   string
		mutate func([]byte) ([]byte, error)
	}{
		{
			name: "extra TAR entry",
			mutate: func(original []byte) ([]byte, error) {
				var output bytes.Buffer
				tw := tar.NewWriter(&output)
				tr := tar.NewReader(bytes.NewReader(original))

				for {
					header, err := tr.Next()
					if err == io.EOF {
						break
					}
					if err != nil {
						return nil, err
					}
					if err := tw.WriteHeader(header); err != nil {
						return nil, err
					}
					if _, err := io.Copy(tw, tr); err != nil {
						return nil, err
					}
				}

				data := []byte("unexpected")
				if err := tw.WriteHeader(&tar.Header{
					Name:     "unexpected.txt",
					Mode:     0600,
					Size:     int64(len(data)),
					Typeflag: tar.TypeReg,
				}); err != nil {
					return nil, err
				}
				if _, err := tw.Write(data); err != nil {
					return nil, err
				}
				if err := tw.Close(); err != nil {
					return nil, err
				}
				return output.Bytes(), nil
			},
		},
		{
			name: "wrong TAR entry order",
			mutate: func(original []byte) ([]byte, error) {
				tr := tar.NewReader(bytes.NewReader(original))

				type entry struct {
					header *tar.Header
					data   []byte
				}

				var entries []entry

				for {
					header, err := tr.Next()
					if err == io.EOF {
						break
					}
					if err != nil {
						return nil, err
					}

					data, err := io.ReadAll(tr)
					if err != nil {
						return nil, err
					}

					copied := *header
					entries = append(entries, entry{
						header: &copied,
						data:   data,
					})
				}

				if len(entries) < 3 {
					return nil, fmt.Errorf("not enough TAR entries")
				}

				entries[1], entries[2] = entries[2], entries[1]

				var output bytes.Buffer
				tw := tar.NewWriter(&output)

				for _, e := range entries {
					if err := tw.WriteHeader(e.header); err != nil {
						return nil, err
					}
					if _, err := tw.Write(e.data); err != nil {
						return nil, err
					}
				}

				if err := tw.Close(); err != nil {
					return nil, err
				}
				return output.Bytes(), nil
			},
		},
		{
			name: "workspace symlink",
			mutate: func(original []byte) ([]byte, error) {
				tr := tar.NewReader(bytes.NewReader(original))
				var output bytes.Buffer
				tw := tar.NewWriter(&output)

				for {
					header, err := tr.Next()
					if err == io.EOF {
						break
					}
					if err != nil {
						return nil, err
					}

					data, err := io.ReadAll(tr)
					if err != nil {
						return nil, err
					}

					copied := *header

					if copied.Name == "workspace/main.go" {
						copied.Typeflag = tar.TypeSymlink
						copied.Linkname = "../../etc/passwd"
						copied.Size = 0
						data = nil
					}

					if err := tw.WriteHeader(&copied); err != nil {
						return nil, err
					}

					if _, err := tw.Write(data); err != nil {
						return nil, err
					}
				}

				if err := tw.Close(); err != nil {
					return nil, err
				}
				return output.Bytes(), nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest, files, artifacts := makeWorkspaceV3Fixture(t)

			var original bytes.Buffer
			if err := WriteWorkspaceV3(
				&original, manifest, files, artifacts,
			); err != nil {
				t.Fatal(err)
			}

			mutated, err := tt.mutate(original.Bytes())
			if err != nil {
				t.Fatal(err)
			}

			capsulePath := filepath.Join(
				t.TempDir(), "invalid.bfc",
			)

			if err := os.WriteFile(capsulePath, mutated, 0600); err != nil {
				t.Fatal(err)
			}

			if _, err := ReadVerifiedV3(capsulePath); err == nil {
				t.Fatal("invalid TAR structure accepted")
			}
		})
	}
}
