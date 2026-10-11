package capsule

import (
	"encoding/json"
	"testing"
)

func TestManifestV3GraphModFilesRoundTrip(t *testing.T) {
	m := validManifestV3()
	m.GoGraphModFiles = validGoGraphModFilesV3()

	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}

	var decoded Manifest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if len(decoded.GoGraphModFiles) != 2 {
		t.Fatalf("graph mod files = %d, want 2", len(decoded.GoGraphModFiles))
	}

	if err := decoded.Validate(); err != nil {
		t.Fatalf("valid graph metadata rejected: %v", err)
	}
}

func TestManifestV3RejectsGraphModFilesInV2(t *testing.T) {
	m := validManifestV3()
	m.SchemaVersion = SchemaVersionV2
	m.GoModules = nil
	m.GoGraphModFiles = validGoGraphModFilesV3()

	if err := m.Validate(); err == nil {
		t.Fatal("graph metadata accepted in schema v2")
	}
}

func TestManifestV3LegacyWithoutGraphModFiles(t *testing.T) {
	m := validManifestV3()

	if err := m.Validate(); err != nil {
		t.Fatalf("legacy manifest rejected: %v", err)
	}

	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}

	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if _, ok := decoded["go_graph_mod_files"]; ok {
		t.Fatal("legacy manifest unexpectedly contains graph metadata")
	}
}
