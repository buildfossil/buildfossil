package capsule

import (
	"encoding/json"
	"fmt"
	"testing"
)

func graphInfoManifestFixtureV3() Manifest {
	m := validManifestV3()
	m.GoModules = nil
	m.GoSelectedModules = selectedModulesFixtureV3(17)

	for i, mod := range m.GoSelectedModules {
		data := []byte(fmt.Sprintf(
			`{"Version":%q}`, mod.Version,
		))

		m.GoGraphInfoFiles = append(
			m.GoGraphInfoFiles,
			GoGraphInfoFileV3{
				Path:    mod.Path,
				Version: mod.Version,
				Artifact: GoModuleArtifact{
					Path:   fmt.Sprintf("graph-info-%04d.info", i+1),
					Size:   int64(len(data)),
					SHA256: SHA256(data),
				},
			},
		)
	}

	return m
}

func TestManifestGoGraphInfoV3Valid(t *testing.T) {
	m := graphInfoManifestFixtureV3()

	if err := m.Validate(); err != nil {
		t.Fatalf("valid graph info manifest rejected: %v", err)
	}

	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}

	var restored Manifest
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}

	if err := restored.Validate(); err != nil {
		t.Fatalf("JSON roundtrip invalid: %v", err)
	}

	if len(restored.GoGraphInfoFiles) != 17 {
		t.Fatalf(
			"roundtrip infos = %d, want 17",
			len(restored.GoGraphInfoFiles),
		)
	}
}

func TestManifestGoGraphInfoV3RejectsMissingInfo(t *testing.T) {
	m := graphInfoManifestFixtureV3()
	m.GoGraphInfoFiles = m.GoGraphInfoFiles[:16]

	if err := m.Validate(); err == nil {
		t.Fatal("incomplete selected metadata accepted")
	}
}

func TestManifestGoGraphInfoV3LegacyCompatibility(t *testing.T) {
	m := validManifestV3()

	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}

	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}

	if _, exists := fields["go_graph_info_files"]; exists {
		t.Fatal("legacy manifest unexpectedly includes graph infos")
	}
}

func TestManifestGoGraphInfoV3RejectsSchemaV2(t *testing.T) {
	m := graphInfoManifestFixtureV3()
	m.SchemaVersion = SchemaVersionV2

	if err := m.Validate(); err == nil {
		t.Fatal("schema-v2 accepted graph info files")
	}
}
