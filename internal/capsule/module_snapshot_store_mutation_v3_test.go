package capsule

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type mutateArtifactOnWriteV3 struct {
	target  string
	changed bool
	output  bytes.Buffer
}

func (w *mutateArtifactOnWriteV3) Write(p []byte) (int, error) {
	if !w.changed {
		w.changed = true

		original, err := os.ReadFile(w.target)
		if err != nil {
			return 0, err
		}

		replacement := bytes.Repeat([]byte{'X'}, len(original))
		if err := os.WriteFile(w.target, replacement, 0600); err != nil {
			return 0, err
		}
	}

	return w.output.Write(p)
}

func TestModuleSnapshotFromStoreV3DetectsMutationDuringCopy(t *testing.T) {
	filename := extendedStoredReaderFixtureV3(t)

	stored, err := ReadVerifiedToStoreV3(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer stored.Close()

	if len(stored.Manifest.GoModules) == 0 {
		t.Fatal("fixture has no Go modules")
	}

	name := "module-0001.zip"
	if _, ok := stored.Store.names[name]; !ok {
		t.Fatalf("fixture missing first streamed artifact %q", name)
	}

	writer := &mutateArtifactOnWriteV3{
		target: filepath.Join(stored.Store.dir, name),
	}

	err = WriteGoModuleSnapshotFromStoreV3(writer, stored)
	if err == nil {
		t.Fatal("SECURITY FAILURE: mutated artifact accepted")
	}

	if !writer.changed {
		t.Fatal("mutation hook was not executed")
	}
	if !strings.Contains(err.Error(), "changed during snapshot copy") {
		t.Fatalf("expected streaming digest rejection, got: %v", err)
	}
	t.Logf("verified rejection: %v", err)
}
