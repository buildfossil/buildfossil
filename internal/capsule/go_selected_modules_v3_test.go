package capsule

import (
	"encoding/json"
	"fmt"
	"testing"
)

func selectedModulesFixtureV3(count int) []GoModuleIdentityV3 {
	result := make([]GoModuleIdentityV3, count)
	for i := range result {
		result[i] = GoModuleIdentityV3{
			Path:    fmt.Sprintf("example.com/mod%03d", i),
			Version: "v1.0.0",
		}
	}
	return result
}

func attachGraphInfoFixturesV3(m *Manifest) {
	for i, mod := range m.GoSelectedModules {
		data := []byte(fmt.Sprintf(`{"Version":%q}`, mod.Version))

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
}

func TestGoSelectedModulesV3LegacyCompatibility(t *testing.T) {
	m := validManifestV3()

	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}

	effective := EffectiveGoSelectedModulesV3(m)

	if len(effective) != len(m.GoModules) {
		t.Fatalf("legacy graph length mismatch")
	}

	for i, mod := range m.GoModules {
		if effective[i].Path != mod.Path ||
			effective[i].Version != mod.Version {
			t.Fatalf("legacy graph mismatch at index %d", i)
		}
	}

	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}

	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if _, exists := decoded["go_selected_modules"]; exists {
		t.Fatal("legacy manifest unexpectedly includes new field")
	}
}

func TestGoSelectedModulesV3Seventeen(t *testing.T) {
	m := validManifestV3()
	m.GoModules = nil
	m.GoSelectedModules = selectedModulesFixtureV3(17)
	attachGraphInfoFixturesV3(&m)

	if err := m.Validate(); err != nil {
		t.Fatalf("17 selected modules rejected: %v", err)
	}

	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}

	var decoded Manifest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if err := decoded.Validate(); err != nil {
		t.Fatalf("JSON roundtrip validation failed: %v", err)
	}

	if len(decoded.GoSelectedModules) != 17 {
		t.Fatalf("roundtrip selected count = %d", len(decoded.GoSelectedModules))
	}
}

func TestGoSelectedModulesV3Boundary(t *testing.T) {
	for _, tc := range []struct {
		count  int
		reject bool
	}{
		{16, false},
		{17, false},
		{64, false},
		{65, true},
	} {
		t.Run(fmt.Sprint(tc.count), func(t *testing.T) {
			m := validManifestV3()
			m.GoModules = nil
			m.GoSelectedModules = selectedModulesFixtureV3(tc.count)
			attachGraphInfoFixturesV3(&m)

			err := m.Validate()
			if (err != nil) != tc.reject {
				t.Fatalf("count %d: err=%v", tc.count, err)
			}
		})
	}
}

func TestGoSelectedModulesV3RejectsInvalid(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Manifest)
	}{
		{
			"missing full module",
			func(m *Manifest) {
				m.GoSelectedModules = selectedModulesFixtureV3(17)
			},
		},
		{
			"wrong full module version",
			func(m *Manifest) {
				m.GoSelectedModules = []GoModuleIdentityV3{
					{Path: m.GoModules[0].Path, Version: "v1.9.0"},
				}
			},
		},
		{
			"duplicate selected path",
			func(m *Manifest) {
				m.GoModules = nil
				m.GoSelectedModules = []GoModuleIdentityV3{
					{Path: "example.com/a", Version: "v1.0.0"},
					{Path: "example.com/a", Version: "v1.1.0"},
				}
			},
		},
		{
			"unsorted selected modules",
			func(m *Manifest) {
				m.GoModules = nil
				m.GoSelectedModules = []GoModuleIdentityV3{
					{Path: "example.com/z", Version: "v1.0.0"},
					{Path: "example.com/a", Version: "v1.0.0"},
				}
			},
		},
		{
			"invalid selected identity",
			func(m *Manifest) {
				m.GoModules = nil
				m.GoSelectedModules = []GoModuleIdentityV3{
					{Path: "../escape", Version: "v1.0.0"},
				}
			},
		},
		{
			"selected modules in schema v2",
			func(m *Manifest) {
				m.SchemaVersion = SchemaVersionV2
				m.GoModules = nil
				m.GoSelectedModules = selectedModulesFixtureV3(17)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := validManifestV3()
			tc.mutate(&m)

			if err := m.Validate(); err == nil {
				t.Fatal("invalid selected module graph accepted")
			}
		})
	}
}
