package capsule

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestVerifyGoModuleZipContentsAcceptsRealisticLargeModuleV3(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "large-module.zip")

	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}

	zw := zip.NewWriter(f)
	w, err := zw.Create("example.com/large@v1.0.0/data.bin")
	if err != nil {
		t.Fatal(err)
	}

	chunk := bytes.Repeat([]byte("x"), 1<<20)

	for i := 0; i < 40; i++ {
		if _, err := w.Write(chunk); err != nil {
			t.Fatal(err)
		}
	}

	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	if err := verifyGoModuleZipContents(zipPath); err != nil {
		t.Fatalf("valid 40 MiB unpacked module rejected: %v", err)
	}
}
