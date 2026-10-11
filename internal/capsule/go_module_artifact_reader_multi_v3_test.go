package capsule

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/mod/module"
)

func TestReadGoModuleArtifactSetForCaptureV3TwoModules(t *testing.T) {
	cache := t.TempDir()

	first, firstArtifacts, firstSum := makeIndexedGoModuleFixture(
		t, 1, "example.com/a",
	)
	second, secondArtifacts, secondSum := makeIndexedGoModuleFixture(
		t, 2, "example.com/b",
	)

	for i, fixture := range []struct {
		mod       GoModuleV3
		artifacts map[string][]byte
	}{
		{first, firstArtifacts},
		{second, secondArtifacts},
	} {
		escapedPath, err := module.EscapePath(fixture.mod.Path)
		if err != nil {
			t.Fatal(err)
		}

		escapedVersion, err := module.EscapeVersion(fixture.mod.Version)
		if err != nil {
			t.Fatal(err)
		}

		dir := filepath.Join(
			cache, "cache", "download",
			filepath.FromSlash(escapedPath), "@v",
		)

		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}

		for _, ext := range []string{".zip", ".mod", ".info"} {
			name := filepath.Join(dir, escapedVersion+ext)
			data := fixture.artifacts[fixture.mod.Artifacts[0].Path]

			for _, artifact := range fixture.mod.Artifacts {
				if filepath.Ext(artifact.Path) == ext {
					data = fixture.artifacts[artifact.Path]
					break
				}
			}

			if err := os.WriteFile(name, data, 0600); err != nil {
				t.Fatal(err)
			}
		}

		_ = i
	}

	identities := []GoModuleIdentityV3{
		{Path: first.Path, Version: first.Version},
		{Path: second.Path, Version: second.Version},
	}

	goSum := append(append([]byte(nil), firstSum...), secondSum...)

	got, artifacts, err := ReadGoModuleArtifactSetForCaptureV3(
		identities, goSum, cache,
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 2 || len(artifacts) != 6 {
		t.Fatalf("unexpected result: %d modules, %d artifacts",
			len(got), len(artifacts))
	}

	if err := VerifyGoModuleArtifactSetV3(
		got, artifacts, goSum,
	); err != nil {
		t.Fatalf("returned artifacts failed verification: %v", err)
	}
}
