package capsule

import (
	"fmt"
	"testing"
)

func multiModuleFixture(index int, path string) GoModuleV3 {
	prefix := fmt.Sprintf("module-%04d", index)

	return GoModuleV3{
		Path:    path,
		Version: "v1.0.0",
		Artifacts: []GoModuleArtifact{
			{Path: prefix + ".zip", Size: 100, SHA256: SHA256([]byte("zip"))},
			{Path: prefix + ".mod", Size: 30, SHA256: SHA256([]byte("mod"))},
			{Path: prefix + ".info", Size: 50, SHA256: SHA256([]byte("info"))},
		},
	}
}

func TestMultiModuleV3AcceptsZeroOneAndTwo(t *testing.T) {
	cases := []struct {
		name    string
		modules []GoModuleV3
	}{
		{"zero", nil},
		{"one", []GoModuleV3{
			multiModuleFixture(1, "example.com/a"),
		}},
		{"two", []GoModuleV3{
			multiModuleFixture(1, "example.com/a"),
			multiModuleFixture(2, "example.com/b"),
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateGoModulesV3(tc.modules); err != nil {
				t.Fatalf("valid modules rejected: %v", err)
			}
		})
	}
}

func TestMultiModuleV3RejectsDuplicateIdentity(t *testing.T) {
	modules := []GoModuleV3{
		multiModuleFixture(1, "example.com/a"),
		multiModuleFixture(2, "example.com/a"),
	}

	if err := ValidateGoModulesV3(modules); err == nil {
		t.Fatal("duplicate module identity accepted")
	}
}

func TestMultiModuleV3RejectsIncorrectArtifactIndex(t *testing.T) {
	modules := []GoModuleV3{
		multiModuleFixture(1, "example.com/a"),
		multiModuleFixture(1, "example.com/b"),
	}

	if err := ValidateGoModulesV3(modules); err == nil {
		t.Fatal("duplicate artifact index accepted")
	}
}

func TestMultiModuleV3RejectsNonCanonicalOrder(t *testing.T) {
	modules := []GoModuleV3{
		multiModuleFixture(1, "example.com/b"),
		multiModuleFixture(2, "example.com/a"),
	}

	if err := ValidateGoModulesV3(modules); err == nil {
		t.Fatal("non-canonical module order accepted")
	}
}

func TestMultiModuleV3AcceptsEightModules(t *testing.T) {
	modules := make([]GoModuleV3, 8)
	for i := range modules {
		modules[i] = multiModuleFixture(
			i+1,
			fmt.Sprintf("example.com/module%02d", i+1),
		)
	}

	if err := ValidateGoModulesV3(modules); err != nil {
		t.Fatalf("eight modules rejected: %v", err)
	}
}

func TestMultiModuleV3RejectsSixtyFiveModules(t *testing.T) {
	modules := make([]GoModuleV3, 65)
	for i := range modules {
		modules[i] = multiModuleFixture(
			i+1,
			fmt.Sprintf("example.com/module%02d", i+1),
		)
	}

	if err := ValidateGoModulesV3(modules); err == nil {
		t.Fatal("sixty-five modules accepted")
	}
}

func TestMultiModuleV3RejectsTotalSizeOverflow(t *testing.T) {
	modules := []GoModuleV3{
		multiModuleFixture(1, "example.com/a"),
		multiModuleFixture(2, "example.com/b"),
	}

	modules[0].Artifacts[0].Size = MaxGoModuleTotalSizeV3

	if err := ValidateGoModulesV3(modules); err == nil {
		t.Fatal("oversized module artifacts accepted")
	}
}
