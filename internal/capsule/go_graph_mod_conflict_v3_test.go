package capsule

import (
	"strings"
	"testing"
)

func TestSplitGoArtifactsV3RejectsIdentityConflict(t *testing.T) {
	modules := []GoModuleV3{
		{
			Path:    "example.com/dep",
			Version: "v1.0.0",
		},
	}

	graphFiles := []GoGraphModFileV3{
		{
			Path:    "example.com/dep",
			Version: "v1.0.0",
			Artifact: GoModuleArtifact{
				Path:   "graph-mod-0001.mod",
				Size:   0,
				SHA256: SHA256(nil),
			},
		},
	}

	_, _, err := SplitGoArtifactsV3(
		modules,
		graphFiles,
		map[string][]byte{},
	)

	if err == nil {
		t.Fatal("selected/graph module identity conflict accepted")
	}

	if !strings.Contains(err.Error(), "duplicates selected module") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSplitGoArtifactsV3RejectsDuplicateGraphIdentity(t *testing.T) {
	files := []GoGraphModFileV3{
		{
			Path:    "example.com/dep",
			Version: "v1.0.0",
		},
		{
			Path:    "example.com/dep",
			Version: "v1.0.0",
		},
	}

	_, _, err := SplitGoArtifactsV3(
		nil,
		files,
		map[string][]byte{},
	)

	if err == nil {
		t.Fatal("duplicate graph module identity accepted")
	}
}
