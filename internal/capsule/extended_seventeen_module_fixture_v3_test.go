package capsule

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	"golang.org/x/mod/sumdb/dirhash"
)

// makeExtendedSeventeenModuleWorkspaceV3 constructs a synthetic
// offline Go module graph with 17 selected modules:
// 5 complete ZIP modules and 12 metadata-only modules.
func makeExtendedSeventeenModuleWorkspaceV3(
	t *testing.T,
) (Manifest, []WorkspaceFile, map[string][]byte) {
	t.Helper()

	manifest := validManifestV3()
	manifest.GoModules = nil
	manifest.GoSelectedModules = make([]GoModuleIdentityV3, 0, 17)

	artifacts := make(map[string][]byte)

	var goMod strings.Builder
	var goSum strings.Builder

	goMod.WriteString("module example.com/app\n\ngo 1.26.6\n\nrequire (\n")

	for i := 1; i <= 17; i++ {
		modulePath := fmt.Sprintf("example.com/module%02d", i)
		version := "v1.0.0"

		manifest.GoSelectedModules = append(
			manifest.GoSelectedModules,
			GoModuleIdentityV3{
				Path:    modulePath,
				Version: version,
			},
		)

		fmt.Fprintf(
			&goMod, "\t%s %s\n", modulePath, version,
		)

		if i <= 5 {
			mod, moduleArtifacts, moduleSum :=
				makeIndexedGoModuleFixture(t, i, modulePath)

			manifest.GoModules = append(manifest.GoModules, mod)

			for name, data := range moduleArtifacts {
				artifacts[name] = data
			}

			goSum.Write(moduleSum)
			continue
		}

		modData := []byte(
			"module " + modulePath + "\n\ngo 1.23\n",
		)

		modHash, err := dirhash.Hash1(
			[]string{"go.mod"},
			func(name string) (io.ReadCloser, error) {
				if name != "go.mod" {
					return nil, fmt.Errorf(
						"unexpected hash input %q", name,
					)
				}

				return io.NopCloser(bytes.NewReader(modData)), nil
			},
		)
		if err != nil {
			t.Fatal(err)
		}

		fmt.Fprintf(
			&goSum,
			"%s %s/go.mod %s\n",
			modulePath, version, modHash,
		)

		graphModName := fmt.Sprintf(
			"graph-mod-%04d.mod", i-5,
		)

		manifest.GoGraphModFiles = append(
			manifest.GoGraphModFiles,
			GoGraphModFileV3{
				Path:    modulePath,
				Version: version,
				Artifact: GoModuleArtifact{
					Path:   graphModName,
					Size:   int64(len(modData)),
					SHA256: SHA256(modData),
				},
			},
		)

		artifacts[graphModName] = modData

		infoData := []byte(`{"Version":"v1.0.0"}`)
		infoName := fmt.Sprintf(
			"graph-info-%04d.info", i-5,
		)

		manifest.GoGraphInfoFiles = append(
			manifest.GoGraphInfoFiles,
			GoGraphInfoFileV3{
				Path:    modulePath,
				Version: version,
				Artifact: GoModuleArtifact{
					Path:   infoName,
					Size:   int64(len(infoData)),
					SHA256: SHA256(infoData),
				},
			},
		)

		artifacts[infoName] = infoData
	}

	goMod.WriteString(")\n")

	files := []WorkspaceFile{
		{
			Path: "go.mod",
			Data: []byte(goMod.String()),
			Mode: 0644,
		},
		{
			Path: "go.sum",
			Data: []byte(goSum.String()),
			Mode: 0644,
		},
		{
			Path: "main.go",
			Data: []byte(
				"package main\nfunc main() { undefinedSymbol() }\n",
			),
			Mode: 0644,
		},
	}

	return manifest, files, artifacts
}
