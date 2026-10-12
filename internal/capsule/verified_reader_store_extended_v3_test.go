package capsule

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func extendedStoredReaderFixtureV3(t *testing.T) string {
	t.Helper()

	manifest, files, artifacts :=
		makeExtendedSeventeenModuleWorkspaceV3(t)

	if len(manifest.GoSelectedModules) != 17 ||
		len(manifest.GoModules) != 5 ||
		len(manifest.GoGraphModFiles) != 12 ||
		len(manifest.GoGraphInfoFiles) != 12 {
		t.Fatal("unexpected extended fixture structure")
	}

	var archive bytes.Buffer
	if err := WriteWorkspaceV3(
		&archive, manifest, files, artifacts,
	); err != nil {
		t.Fatalf("write extended capsule: %v", err)
	}

	filename := filepath.Join(t.TempDir(), "extended.bfc")
	if err := os.WriteFile(
		filename, archive.Bytes(), 0600,
	); err != nil {
		t.Fatal(err)
	}

	return filename
}

func TestReadVerifiedToStoreV3ExtendedRoundTrip(t *testing.T) {
	filename := extendedStoredReaderFixtureV3(t)

	legacy, err := ReadVerifiedV3(filename)
	if err != nil {
		t.Fatalf("legacy reader: %v", err)
	}

	stored, err := ReadVerifiedToStoreV3(filename)
	if err != nil {
		t.Fatalf("disk reader: %v", err)
	}
	defer stored.Close()

	if !reflect.DeepEqual(
		legacy.Manifest, stored.Manifest,
	) {
		t.Fatal("manifests differ")
	}

	if !reflect.DeepEqual(
		legacy.Files, stored.Files,
	) {
		t.Fatal("workspace files differ")
	}

	if len(legacy.Artifacts) != len(stored.Store.names) {
		t.Fatalf(
			"artifact count mismatch: legacy=%d stored=%d",
			len(legacy.Artifacts),
			len(stored.Store.names),
		)
	}

	if len(legacy.Artifacts) != 39 {
		t.Fatalf(
			"expected 39 artifacts, got %d",
			len(legacy.Artifacts),
		)
	}

	for name, expected := range legacy.Artifacts {
		file, err := stored.Store.Open(name)
		if err != nil {
			t.Fatalf("open %s: %v", name, err)
		}

		actual, readErr := io.ReadAll(file)
		closeErr := file.Close()

		if readErr != nil || closeErr != nil {
			t.Fatalf(
				"read %s: %v; close: %v",
				name, readErr, closeErr,
			)
		}

		if !bytes.Equal(actual, expected) {
			t.Fatalf("artifact mismatch: %s", name)
		}
	}

	directory := stored.Store.dir
	if err := stored.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(directory); !os.IsNotExist(err) {
		t.Fatalf("store directory remains: %v", err)
	}
}

func TestReadVerifiedToStoreV3ExtendedGraphTampering(
	t *testing.T,
) {
	filename := extendedStoredReaderFixtureV3(t)

	legacy, err := ReadVerifiedV3(filename)
	if err != nil {
		t.Fatal(err)
	}

	encoded, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}

	scenarios := []struct {
		name string
		path string
	}{
		{
			name: "graph mod",
			path: legacy.Manifest.GoGraphModFiles[0].Artifact.Path,
		},
		{
			name: "graph info",
			path: legacy.Manifest.GoGraphInfoFiles[0].Artifact.Path,
		},
	}

	for _, tc := range scenarios {
		t.Run(tc.name, func(t *testing.T) {
			artifact := legacy.Artifacts[tc.path]
			if len(artifact) == 0 {
				t.Fatalf("empty fixture artifact: %s", tc.path)
			}

			offset := bytes.Index(encoded, artifact)
			if offset < 0 {
				t.Fatalf(
					"artifact missing from TAR: %s", tc.path,
				)
			}

			corrupted := append([]byte(nil), encoded...)
			corrupted[offset+len(artifact)/2] ^= 1

			bad := filepath.Join(
				t.TempDir(), "corrupt-graph.bfc",
			)
			if err := os.WriteFile(
				bad, corrupted, 0600,
			); err != nil {
				t.Fatal(err)
			}

			if _, err := ReadVerifiedV3(bad); err == nil {
				t.Fatal(
					"legacy reader accepted corrupted graph artifact",
				)
			}

			tempRoot := t.TempDir()
			t.Setenv("TMPDIR", tempRoot)

			stored, err := ReadVerifiedToStoreV3(bad)
			if err == nil {
				_ = stored.Close()
				t.Fatal(
					"disk reader accepted corrupted graph artifact",
				)
			}

			remaining, err := os.ReadDir(tempRoot)
			if err != nil {
				t.Fatal(err)
			}

			if len(remaining) != 0 {
				t.Fatalf(
					"temporary artifacts leaked: %v",
					remaining,
				)
			}
		})
	}
}
