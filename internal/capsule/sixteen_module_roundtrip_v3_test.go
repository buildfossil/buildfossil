package capsule

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteReadV3SixteenModulesRoundTrip(t *testing.T) {
	manifest, files, artifacts := makeEightModuleWorkspaceV3(t)

	for index := 9; index <= 16; index++ {
		modulePath := fmt.Sprintf("example.com/module%02d", index)

		mod, moduleArtifacts, moduleSum := makeIndexedGoModuleFixture(
			t, index, modulePath,
		)

		manifest.GoModules = append(manifest.GoModules, mod)

		for name, data := range moduleArtifacts {
			artifacts[name] = data
		}

		old := []byte(")\n")
		replacement := []byte(fmt.Sprintf(
			"\t%s v1.0.0\n)\n", modulePath,
		))

		updated := bytes.Replace(files[0].Data, old, replacement, 1)
		if bytes.Equal(updated, files[0].Data) {
			t.Fatal("failed to update go.mod")
		}
		files[0].Data = updated
		files[1].Data = append(files[1].Data, moduleSum...)
	}

	var output bytes.Buffer

	if err := WriteWorkspaceV3(
		&output, manifest, files, artifacts,
	); err != nil {
		t.Fatalf("write sixteen-module capsule: %v", err)
	}

	if int64(output.Len()) > MaxCapsuleSize {
		t.Fatalf("capsule exceeds size limit: %d", output.Len())
	}

	filename := filepath.Join(t.TempDir(), "sixteen.bfc")

	if err := os.WriteFile(filename, output.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}

	verified, err := ReadVerifiedV3(filename)
	if err != nil {
		t.Fatalf("read sixteen-module capsule: %v", err)
	}

	if got := len(verified.Manifest.GoModules); got != 16 {
		t.Fatalf("module count = %d; want 16", got)
	}

	if got := len(verified.Artifacts); got != 48 {
		t.Fatalf("artifact count = %d; want 48", got)
	}

	if err := VerifyGoModuleArtifactSetV3(
		verified.Manifest.GoModules,
		verified.Artifacts,
		files[1].Data,
	); err != nil {
		t.Fatalf("verify sixteen-module artifacts: %v", err)
	}

	t.Logf("capsule size: %d bytes", output.Len())
}
