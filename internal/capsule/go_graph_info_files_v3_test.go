package capsule

import (
	"strings"
	"testing"
)

func validGraphInfoFixtureV3() (
	GoGraphInfoFileV3,
	[]GoModuleIdentityV3,
	[]byte,
) {
	data := []byte(`{"Version":"v1.0.0"}`)

	file := GoGraphInfoFileV3{
		Path:    "example.com/a",
		Version: "v1.0.0",
		Artifact: GoModuleArtifact{
			Path:   "graph-info-0001.info",
			Size:   int64(len(data)),
			SHA256: SHA256(data),
		},
	}

	selected := []GoModuleIdentityV3{
		{Path: "example.com/a", Version: "v1.0.0"},
	}

	return file, selected, data
}

func TestGoGraphInfoFilesV3Valid(t *testing.T) {
	file, selected, data := validGraphInfoFixtureV3()

	if err := ValidateGoGraphInfoFilesV3(
		[]GoGraphInfoFileV3{file},
		selected,
		nil,
	); err != nil {
		t.Fatal(err)
	}

	if err := VerifyGoGraphInfoFileV3(file, data); err != nil {
		t.Fatal(err)
	}
}

func TestGoGraphInfoFilesV3RejectsInvalid(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*GoGraphInfoFileV3, *[]GoModuleIdentityV3)
	}{
		{
			name: "module absent from selected graph",
			mutate: func(
				f *GoGraphInfoFileV3,
				s *[]GoModuleIdentityV3,
			) {
				*s = nil
			},
		},
		{
			name: "wrong selected version",
			mutate: func(
				f *GoGraphInfoFileV3,
				s *[]GoModuleIdentityV3,
			) {
				(*s)[0].Version = "v1.1.0"
			},
		},
		{
			name: "invalid module path",
			mutate: func(
				f *GoGraphInfoFileV3,
				s *[]GoModuleIdentityV3,
			) {
				f.Path = "../escape"
			},
		},
		{
			name: "wrong artifact name",
			mutate: func(
				f *GoGraphInfoFileV3,
				s *[]GoModuleIdentityV3,
			) {
				f.Artifact.Path = "graph-info-9999.info"
			},
		},
		{
			name: "invalid checksum",
			mutate: func(
				f *GoGraphInfoFileV3,
				s *[]GoModuleIdentityV3,
			) {
				f.Artifact.SHA256 = "sha256:" + strings.Repeat("z", 64)
			},
		},
		{
			name: "negative size",
			mutate: func(
				f *GoGraphInfoFileV3,
				s *[]GoModuleIdentityV3,
			) {
				f.Artifact.Size = -1
			},
		},
		{
			name: "oversized artifact",
			mutate: func(
				f *GoGraphInfoFileV3,
				s *[]GoModuleIdentityV3,
			) {
				f.Artifact.Size = MaxGoGraphInfoTotalSizeV3 + 1
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			file, selected, _ := validGraphInfoFixtureV3()
			tc.mutate(&file, &selected)

			if err := ValidateGoGraphInfoFilesV3(
				[]GoGraphInfoFileV3{file},
				selected,
				nil,
			); err == nil {
				t.Fatal("invalid graph info accepted")
			}
		})
	}

	t.Run("duplicate full module", func(t *testing.T) {
		file, selected, _ := validGraphInfoFixtureV3()

		full := []GoModuleV3{
			{Path: file.Path, Version: file.Version},
		}

		if err := ValidateGoGraphInfoFilesV3(
			[]GoGraphInfoFileV3{file},
			selected,
			full,
		); err == nil {
			t.Fatal("duplicate full module accepted")
		}
	})

	t.Run("duplicate graph info", func(t *testing.T) {
		file, selected, _ := validGraphInfoFixtureV3()

		if err := ValidateGoGraphInfoFilesV3(
			[]GoGraphInfoFileV3{file, file},
			selected,
			nil,
		); err == nil {
			t.Fatal("duplicate graph info accepted")
		}
	})
}

func TestVerifyGoGraphInfoFileV3RejectsTampering(t *testing.T) {
	file, _, data := validGraphInfoFixtureV3()

	cases := []struct {
		name string
		data []byte
	}{
		{"content changed", []byte(`{"Version":"v2.0.0"}`)},
		{"invalid JSON", []byte(`not json`)},
		{"empty", nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := VerifyGoGraphInfoFileV3(
				file, tc.data,
			); err == nil {
				t.Fatal("tampered graph info accepted")
			}
		})
	}

	mismatched := file
	mismatched.Version = "v1.1.0"

	if err := VerifyGoGraphInfoFileV3(
		mismatched, data,
	); err == nil {
		t.Fatal("wrong JSON version accepted")
	}
}
