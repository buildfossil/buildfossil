package capsule

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func TestWorkspaceWriterFromStoreV3ExtendedMatchesLegacy(
	t *testing.T,
) {
	manifest, files, artifacts :=
		makeExtendedSeventeenModuleWorkspaceV3(t)

	var legacy bytes.Buffer
	if err := WriteWorkspaceV3(
		&legacy, manifest, files, artifacts,
	); err != nil {
		t.Fatal(err)
	}

	// ReadVerifiedV3 provides a validated manifest with
	// populated workspace metadata.
	filename := extendedStoredReaderFixtureV3(t)
	verified, err := ReadVerifiedV3(filename)
	if err != nil {
		t.Fatal(err)
	}
	store := completeStoreFixtureV3(t, verified)

	var actual bytes.Buffer
	if err := WriteWorkspaceFromStoreV3(
		&actual, manifest, files, store,
	); err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(legacy.Bytes(), actual.Bytes()) {
		t.Fatalf(
			"capsule mismatch: legacy=%d stored=%d",
			legacy.Len(), actual.Len(),
		)
	}
}

func TestWorkspaceWriterFromStoreV3RejectsMissingStore(
	t *testing.T,
) {
	manifest, files, _ :=
		makeExtendedSeventeenModuleWorkspaceV3(t)

	var output bytes.Buffer
	if err := WriteWorkspaceFromStoreV3(
		&output, manifest, files, nil,
	); err == nil {
		t.Fatal("missing store accepted")
	}

	if output.Len() != 0 {
		t.Fatal("invalid input produced output")
	}
}

func TestWorkspaceWriterFromStoreV3RejectsWorkspaceMutation(
	t *testing.T,
) {
	manifest, files, _ :=
		makeExtendedSeventeenModuleWorkspaceV3(t)

	filename := extendedStoredReaderFixtureV3(t)
	verified, err := ReadVerifiedV3(filename)
	if err != nil {
		t.Fatal(err)
	}
	store := completeStoreFixtureV3(t, verified)

	files[0].Data = []byte("invalid go.mod")

	var output bytes.Buffer
	if err := WriteWorkspaceFromStoreV3(
		&output, manifest, files, store,
	); err == nil {
		t.Fatal("invalid go.mod accepted")
	}

	if output.Len() != 0 {
		t.Fatal("invalid workspace produced output")
	}
}

func TestWorkspaceWriterFromStoreV3PropagatesWriterError(
	t *testing.T,
) {
	manifest, files, _ :=
		makeExtendedSeventeenModuleWorkspaceV3(t)

	filename := extendedStoredReaderFixtureV3(t)
	verified, err := ReadVerifiedV3(filename)
	if err != nil {
		t.Fatal(err)
	}
	store := completeStoreFixtureV3(t, verified)

	err = WriteWorkspaceFromStoreV3(
		failingWorkspaceSnapshotWriterV3{},
		manifest, files, store,
	)
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("writer error not propagated: %v", err)
	}
}
