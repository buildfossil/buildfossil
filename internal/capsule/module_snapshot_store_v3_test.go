package capsule

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func TestModuleSnapshotFromStoreV3ExtendedMatchesLegacy(
	t *testing.T,
) {
	filename := extendedStoredReaderFixtureV3(t)

	legacy, err := ReadVerifiedV3(filename)
	if err != nil {
		t.Fatal(err)
	}

	stored, err := ReadVerifiedToStoreV3(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer stored.Close()

	expected, err := BuildGoModuleSnapshotV3(legacy)
	if err != nil {
		t.Fatal(err)
	}

	var actual bytes.Buffer
	if err := WriteGoModuleSnapshotFromStoreV3(
		&actual, stored,
	); err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(expected, actual.Bytes()) {
		t.Fatalf(
			"module TAR differs: legacy=%d bytes stored=%d bytes",
			len(expected), actual.Len(),
		)
	}
}

func TestModuleSnapshotFromStoreV3RejectsTamperedWorkspace(
	t *testing.T,
) {
	filename := extendedStoredReaderFixtureV3(t)

	stored, err := ReadVerifiedToStoreV3(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer stored.Close()

	stored.Files[0].Data = []byte("tampered workspace")

	var output bytes.Buffer
	if err := WriteGoModuleSnapshotFromStoreV3(
		&output, stored,
	); err == nil {
		t.Fatal("tampered workspace accepted")
	}

	if output.Len() != 0 {
		t.Fatal("invalid workspace produced TAR output")
	}
}

func TestModuleSnapshotFromStoreV3RejectsMissingStore(
	t *testing.T,
) {
	filename := extendedStoredReaderFixtureV3(t)

	stored, err := ReadVerifiedToStoreV3(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer stored.Close()

	stored.Store = nil

	var output bytes.Buffer
	if err := WriteGoModuleSnapshotFromStoreV3(
		&output, stored,
	); err == nil {
		t.Fatal("nil artifact store accepted")
	}

	if output.Len() != 0 {
		t.Fatal("nil store produced TAR output")
	}
}

func TestModuleSnapshotFromStoreV3PropagatesWriterError(
	t *testing.T,
) {
	filename := extendedStoredReaderFixtureV3(t)

	stored, err := ReadVerifiedToStoreV3(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer stored.Close()

	err = WriteGoModuleSnapshotFromStoreV3(
		failingWorkspaceSnapshotWriterV3{},
		stored,
	)

	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf(
			"writer failure not propagated: %v", err,
		)
	}
}
