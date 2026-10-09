package capsule

import (
	"archive/tar"
	"bytes"
	"testing"
)

func TestExpectedTarSize(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	content := []byte("hello")

	if err := tw.WriteHeader(&tar.Header{
		Name:     "manifest.json",
		Mode:     0600,
		Size:     int64(len(content)),
		Typeflag: tar.TypeReg,
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}

	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	got := expectedTarSize(int64(len(content)), Workspace{})
	want := int64(buf.Len())

	if got != want {
		t.Fatalf("expected tar size %d, actual size %d", got, want)
	}
}
