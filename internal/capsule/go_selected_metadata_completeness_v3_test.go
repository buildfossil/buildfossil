package capsule

import (
	"fmt"
	"testing"
)

func TestGoSelectedMetadataCompletenessV3(t *testing.T) {
	selected := selectedModulesFixtureV3(17)

	full := make([]GoModuleV3, 0, 5)
	for _, mod := range selected[:5] {
		full = append(full, GoModuleV3{
			Path: mod.Path, Version: mod.Version,
		})
	}

	infos := make([]GoGraphInfoFileV3, 0, 12)
	for i, mod := range selected[5:] {
		data := []byte(fmt.Sprintf(
			`{"Version":%q}`, mod.Version,
		))

		infos = append(infos, GoGraphInfoFileV3{
			Path:    mod.Path,
			Version: mod.Version,
			Artifact: GoModuleArtifact{
				Path:   fmt.Sprintf("graph-info-%04d.info", i+1),
				Size:   int64(len(data)),
				SHA256: SHA256(data),
			},
		})
	}

	t.Run("17 selected 5 full 12 metadata", func(t *testing.T) {
		if err := ValidateGoSelectedMetadataCompletenessV3(
			selected, full, infos,
		); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("missing one metadata artifact", func(t *testing.T) {
		if err := ValidateGoSelectedMetadataCompletenessV3(
			selected, full, infos[:11],
		); err == nil {
			t.Fatal("incomplete metadata accepted")
		}
	})

	t.Run("duplicate full representation", func(t *testing.T) {
		bad := append([]GoGraphInfoFileV3(nil), infos...)
		bad[0].Path = full[0].Path
		bad[0].Version = full[0].Version

		if err := ValidateGoSelectedMetadataCompletenessV3(
			selected, full, bad,
		); err == nil {
			t.Fatal("duplicate module representation accepted")
		}
	})

	t.Run("legacy full ZIP only", func(t *testing.T) {
		if err := ValidateGoSelectedMetadataCompletenessV3(
			nil, full, nil,
		); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("legacy rejects new info files", func(t *testing.T) {
		if err := ValidateGoSelectedMetadataCompletenessV3(
			nil, full, infos,
		); err == nil {
			t.Fatal("legacy manifest accepted graph info")
		}
	})

	t.Run("zero dependency graph", func(t *testing.T) {
		if err := ValidateGoSelectedMetadataCompletenessV3(
			[]GoModuleIdentityV3{}, nil, nil,
		); err != nil {
			t.Fatal(err)
		}
	})
}
