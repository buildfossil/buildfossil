package capsule

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestGoModuleSnapshotV3StreamMatchesAPI(t *testing.T) {
	verified := makeVerifiedSnapshotFixture(t)

	legacy, err := BuildGoModuleSnapshotV3(verified)
	if err != nil {
		t.Fatal(err)
	}

	var streamed bytes.Buffer
	if err := WriteGoModuleSnapshotV3(
		&streamed, verified,
	); err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(legacy, streamed.Bytes()) {
		t.Fatal("module TAR differs between APIs")
	}

	// Also exercise a file destination.
	path := filepath.Join(t.TempDir(), "module.tar")
	file, err := os.OpenFile(
		path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	if err := WriteGoModuleSnapshotV3(file, verified); err != nil {
		t.Fatal(err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}

	onDisk, err := io.ReadAll(file)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(legacy, onDisk) {
		t.Fatal("file-backed module TAR differs")
	}
}

func TestGoModuleSnapshotV3StreamRejectsMutation(t *testing.T) {
	verified := makeVerifiedSnapshotFixture(t)
	verified.Files[0].Data = []byte("tampered")

	var output bytes.Buffer
	if err := WriteGoModuleSnapshotV3(
		&output, verified,
	); err == nil {
		t.Fatal("mutated workspace accepted")
	}
	if output.Len() != 0 {
		t.Fatal("invalid workspace produced output")
	}
}

func TestGoModuleSnapshotV3StreamPropagatesWriterError(t *testing.T) {
	verified := makeVerifiedSnapshotFixture(t)

	err := WriteGoModuleSnapshotV3(
		failingWorkspaceSnapshotWriterV3{},
		verified,
	)
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("writer error not propagated: %v", err)
	}
}
