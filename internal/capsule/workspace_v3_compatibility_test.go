package capsule

import (
	"archive/tar"
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestWorkspaceV3RealWorldFileCapacity(t *testing.T) {
	const count = 1081

	files := make([]FileMetadata, 0, count)
	for i := 0; i < count; i++ {
		files = append(files, FileMetadata{
			Path:   fmt.Sprintf("internal/fixtures/file-%04d.toml", i),
			Size:   1,
			SHA256: "sha256:" + strings.Repeat("a", 64),
		})
	}

	if err := validateWorkspaceV3(Workspace{Files: files}); err != nil {
		t.Fatalf("v3 should support %d files: %v", count, err)
	}
}

func TestWorkspaceV3RealWorldUSTARPath(t *testing.T) {
	name := "internal/toml-test/tests/invalid/string/" +
		"basic-multiline-out-of-range-unicode-escape-01.toml"

	if got := len("workspace/" + name); got != 101 {
		t.Fatalf("test path length = %d, want 101", got)
	}

	file := FileMetadata{
		Path:   name,
		Size:   1,
		SHA256: "sha256:" + strings.Repeat("a", 64),
	}

	if err := validateWorkspaceV3(
		Workspace{Files: []FileMetadata{file}},
	); err != nil {
		t.Fatalf("v3 should accept USTAR-compatible path: %v", err)
	}
}

func TestWorkspaceV3USTARRepresentability(t *testing.T) {
	cases := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name: "real toml embed path",
			path: "workspace/internal/toml-test/tests/invalid/string/" +
				"basic-multiline-out-of-range-unicode-escape-01.toml",
			wantErr: false,
		},
		{
			name:    "long single component",
			path:    "workspace/" + strings.Repeat("x", 120),
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			tw := tar.NewWriter(&buf)

			err := tw.WriteHeader(&tar.Header{
				Name:     tc.path,
				Mode:     0600,
				Size:     0,
				Typeflag: tar.TypeReg,
				Format:   tar.FormatUSTAR,
			})

			if (err != nil) != tc.wantErr {
				t.Fatalf("WriteHeader error = %v, wantErr %v",
					err, tc.wantErr)
			}
			if err != nil {
				return
			}

			if err := tw.Close(); err != nil {
				t.Fatal(err)
			}

			tr := tar.NewReader(bytes.NewReader(buf.Bytes()))
			header, err := tr.Next()
			if err != nil {
				t.Fatal(err)
			}

			if header.Name != tc.path {
				t.Fatalf("roundtrip path = %q, want %q",
					header.Name, tc.path)
			}
			if header.Format != tar.FormatUSTAR {
				t.Fatalf("TAR format = %v, want USTAR",
					header.Format)
			}
			if buf.Len() != 1536 {
				t.Fatalf("archive length = %d, want 1536",
					buf.Len())
			}
		})
	}
}
