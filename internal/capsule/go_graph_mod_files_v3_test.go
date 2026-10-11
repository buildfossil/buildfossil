package capsule

import (
	"strings"
	"testing"
)

func validGoGraphModFilesV3() []GoGraphModFileV3 {
	return []GoGraphModFileV3{
		{
			Path:    "github.com/mattn/go-isatty",
			Version: "v0.0.16",
			Artifact: GoModuleArtifact{
				Path:   "graph-mod-0001.mod",
				Size:   42,
				SHA256: "sha256:" + strings.Repeat("a", 64),
			},
		},
		{
			Path:    "golang.org/x/sys",
			Version: "v0.6.0",
			Artifact: GoModuleArtifact{
				Path:   "graph-mod-0002.mod",
				Size:   42,
				SHA256: "sha256:" + strings.Repeat("b", 64),
			},
		},
	}
}

func TestValidateGoGraphModFilesV3(t *testing.T) {
	if err := ValidateGoGraphModFilesV3(nil); err != nil {
		t.Fatalf("empty metadata rejected: %v", err)
	}

	if err := ValidateGoGraphModFilesV3(validGoGraphModFilesV3()); err != nil {
		t.Fatalf("valid metadata rejected: %v", err)
	}
}

func TestValidateGoGraphModFilesV3RejectsInvalid(t *testing.T) {
	tests := []struct {
		name string
		edit func([]GoGraphModFileV3)
	}{
		{
			name: "invalid module path",
			edit: func(files []GoGraphModFileV3) {
				files[0].Path = "../escape"
			},
		},
		{
			name: "duplicate module version",
			edit: func(files []GoGraphModFileV3) {
				files[1].Path = files[0].Path
				files[1].Version = files[0].Version
			},
		},
		{
			name: "unsorted paths",
			edit: func(files []GoGraphModFileV3) {
				files[0], files[1] = files[1], files[0]
			},
		},
		{
			name: "wrong artifact name",
			edit: func(files []GoGraphModFileV3) {
				files[0].Artifact.Path = "../evil.mod"
			},
		},
		{
			name: "negative artifact size",
			edit: func(files []GoGraphModFileV3) {
				files[0].Artifact.Size = -1
			},
		},
		{
			name: "invalid SHA-256",
			edit: func(files []GoGraphModFileV3) {
				files[0].Artifact.SHA256 = "sha256:invalid"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := validGoGraphModFilesV3()
			tt.edit(files)

			if err := ValidateGoGraphModFilesV3(files); err == nil {
				t.Fatal("invalid graph metadata accepted")
			}
		})
	}
}

func TestValidateGoGraphModFilesV3RejectsTooMany(t *testing.T) {
	files := make([]GoGraphModFileV3, 65)

	if err := ValidateGoGraphModFilesV3(files); err == nil {
		t.Fatal("too many graph mod files accepted")
	}
}

func TestValidateGoGraphModFilesV3RejectsOversized(t *testing.T) {
	files := validGoGraphModFilesV3()
	files[0].Artifact.Size = MaxGoGraphModTotalSizeV3

	if err := ValidateGoGraphModFilesV3(files); err == nil {
		t.Fatal("oversized graph metadata accepted")
	}
}

func TestValidateGoGraphModFilesV3RejectsUnsortedVersions(t *testing.T) {
	files := validGoGraphModFilesV3()

	files[0].Path = "golang.org/x/sys"
	files[0].Version = "v0.6.0"

	files[1].Path = "golang.org/x/sys"
	files[1].Version = "v0.0.0-20220811171246-fbc7d0a398ab"

	if err := ValidateGoGraphModFilesV3(files); err == nil {
		t.Fatal("unsorted graph module versions accepted")
	}
}
