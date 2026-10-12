package capsule

import (
	"strings"
	"testing"
)

func TestArchiveSizePreflightV3Boundaries(t *testing.T) {
	// Metadata-only synthetic sizes test the archive ceiling,
	// independently of the workspace and module category limits.
	const manifestSize int64 = 1024

	makeManifest := func(payload int64) Manifest {
		return Manifest{
			Workspace: Workspace{
				Files: []FileMetadata{
					{Path: "synthetic.bin", Size: payload},
				},
			},
		}
	}

	overhead := expectedTarSize(
		manifestSize,
		Workspace{
			Files: []FileMetadata{
				{Path: "synthetic.bin", Size: 0},
			},
		},
	)

	if overhead >= MaxCapsuleSizeV3 {
		t.Fatal("invalid TAR overhead")
	}

	maxPayload := MaxCapsuleSizeV3 - overhead

	tests := []struct {
		name   string
		size   int64
		reject bool
	}{
		{"below", maxPayload - 512, false},
		{"exact", maxPayload, false},
		{"above", maxPayload + 512, true},
		{"negative", -1, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateArchiveSizePreflightV3(
				makeManifest(tc.size), manifestSize,
			)
			if (err != nil) != tc.reject {
				t.Fatalf("size=%d reject=%v err=%v",
					tc.size, tc.reject, err)
			}
		})
	}
}

func TestArchiveSizePreflightV3CountsArtifacts(t *testing.T) {
	manifest := Manifest{
		Workspace: Workspace{
			Files: []FileMetadata{
				{Path: "go.mod", Size: 100},
				{Path: "go.sum", Size: 100},
			},
		},
		GoModules: []GoModuleV3{
			{
				Artifacts: []GoModuleArtifact{
					{Path: "module-0001.zip", Size: MaxCapsuleSizeV3},
				},
			},
		},
	}

	err := validateArchiveSizePreflightV3(manifest, 1024)
	if err == nil || !strings.Contains(err.Error(), "size limit") {
		t.Fatalf("oversized artifacts accepted: %v", err)
	}
}

func TestArchiveSizePreflightV3RejectsNegativeArtifact(t *testing.T) {
	manifest := Manifest{
		GoModules: []GoModuleV3{
			{
				Artifacts: []GoModuleArtifact{
					{Size: -1},
				},
			},
		},
	}

	if err := validateArchiveSizePreflightV3(
		manifest, 1024,
	); err == nil {
		t.Fatal("negative artifact size accepted")
	}
}
