package capsule

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
	"os"
	"testing"
)

func TestWorkspaceSnapshotV3StreamMatchesLegacy(t *testing.T) {
	verified := makeVerifiedSnapshotFixture(t)

	legacy, err := BuildWorkspaceSnapshotV3(verified)
	if err != nil {
		t.Fatal(err)
	}

	var streamed bytes.Buffer
	if err := WriteWorkspaceSnapshotV3(
		&streamed, verified,
	); err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(legacy, streamed.Bytes()) {
		t.Fatal("streaming snapshot differs from legacy API")
	}

	// The expected bytes are also checked independently
	// against TAR entries and metadata below.
	expected := make(map[string][]byte)
	for _, file := range verified.Files {
		expected[file.Path] = file.Data
	}

	reader := tar.NewReader(bytes.NewReader(streamed.Bytes()))
	seen := make(map[string]bool)

	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if header.Typeflag != tar.TypeReg {
			t.Fatalf("unexpected entry type: %q", header.Name)
		}

		want, ok := expected[header.Name]
		if !ok || seen[header.Name] {
			t.Fatalf("unexpected or duplicate entry: %q", header.Name)
		}

		got, err := io.ReadAll(reader)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("content mismatch: %q", header.Name)
		}
		seen[header.Name] = true
	}

	if len(seen) != len(expected) {
		t.Fatalf("got %d entries, want %d", len(seen), len(expected))
	}
}

func TestWorkspaceSnapshotV3StreamRejectsMutationBeforeWrite(t *testing.T) {
	verified := makeVerifiedSnapshotFixture(t)
	verified.Files[0].Data = []byte("tampered")

	var output bytes.Buffer
	if err := WriteWorkspaceSnapshotV3(
		&output, verified,
	); err == nil {
		t.Fatal("tampered workspace accepted")
	}
	if output.Len() != 0 {
		t.Fatal("invalid workspace produced partial TAR")
	}
}

func TestWorkspaceSnapshotV3StreamPropagatesWriterError(t *testing.T) {
	verified := makeVerifiedSnapshotFixture(t)

	err := WriteWorkspaceSnapshotV3(
		failingWorkspaceSnapshotWriterV3{},
		verified,
	)
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("expected writer error, got %v", err)
	}
}

func TestWorkspaceSnapshotV3LimitWriter(t *testing.T) {
	var output bytes.Buffer
	limited := &workspaceSnapshotLimitWriterV3{
		dst: &output, remaining: 5,
	}
	if _, err := limited.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	if _, err := limited.Write([]byte("!")); err == nil {
		t.Fatal("limit overflow accepted")
	}
	if output.String() != "hello" {
		t.Fatalf("unexpected output %q", output.String())
	}
}

type failingWorkspaceSnapshotWriterV3 struct{}

func (failingWorkspaceSnapshotWriterV3) Write([]byte) (int, error) {
	return 0, io.ErrClosedPipe
}

func TestWorkspaceSnapshotV3WriterError(t *testing.T) {
	limited := &workspaceSnapshotLimitWriterV3{
		dst:       failingWorkspaceSnapshotWriterV3{},
		remaining: 100,
	}
	if _, err := limited.Write([]byte("data")); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("wrong error: %v", err)
	}
}

func TestWorkspaceSnapshotV3LimitWriterBoundary(t *testing.T) {
	var output bytes.Buffer
	writer := &workspaceSnapshotLimitWriterV3{
		dst:       &output,
		remaining: 4,
	}

	if n, err := writer.Write([]byte("1234")); err != nil || n != 4 {
		t.Fatalf("exact boundary: n=%d err=%v", n, err)
	}

	if n, err := writer.Write([]byte("5")); err == nil || n != 0 {
		t.Fatalf("overflow accepted: n=%d err=%v", n, err)
	}

	if output.String() != "1234" {
		t.Fatalf("unexpected data: %q", output.String())
	}
}

func TestWorkspaceSnapshotV3LargeWorkspaceStream(t *testing.T) {
	verified := makeVerifiedSnapshotFixture(t)

	// Preserve the manifest and valid workspace entries.
	// Exercise a file-backed destination without changing
	// the capsule's integrity metadata.
	output, err := os.CreateTemp(t.TempDir(), "snapshot-*.tar")
	if err != nil {
		t.Fatal(err)
	}
	defer output.Close()

	if err := WriteWorkspaceSnapshotV3(output, verified); err != nil {
		t.Fatal(err)
	}

	info, err := output.Stat()
	if err != nil {
		t.Fatal(err)
	}

	legacy, err := BuildWorkspaceSnapshotV3(verified)
	if err != nil {
		t.Fatal(err)
	}

	if info.Size() != int64(len(legacy)) {
		t.Fatalf(
			"file-backed size=%d legacy size=%d",
			info.Size(), len(legacy),
		)
	}

	if _, err := output.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}

	streamed, err := io.ReadAll(output)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(streamed, legacy) {
		t.Fatal("file-backed TAR differs from in-memory TAR")
	}
}
