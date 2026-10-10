package capsule

import (
	"strings"
	"testing"
)

func validGoModuleV3() GoModuleV3 {
	digest := "sha256:" + strings.Repeat("a", 64)

	return GoModuleV3{
		Path:    "github.com/google/uuid",
		Version: "v1.6.0",
		Artifacts: []GoModuleArtifact{
			{Path: "module-0001.zip", Size: 100, SHA256: digest},
			{Path: "module-0001.mod", Size: 30, SHA256: digest},
			{Path: "module-0001.info", Size: 50, SHA256: digest},
		},
	}
}

func TestValidateGoModulesV3(t *testing.T) {
	if err := ValidateGoModulesV3([]GoModuleV3{
		validGoModuleV3(),
	}); err != nil {
		t.Fatalf("valid module rejected: %v", err)
	}

	if err := ValidateGoModulesV3(nil); err != nil {
		t.Fatalf("empty modules rejected: %v", err)
	}
}

func TestValidateGoModulesV3RejectsInvalidMetadata(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*GoModuleV3)
	}{
		{
			name: "path traversal",
			mutate: func(m *GoModuleV3) {
				m.Path = "../../evil"
			},
		},
		{
			name: "invalid version",
			mutate: func(m *GoModuleV3) {
				m.Version = "latest"
			},
		},
		{
			name: "major version mismatch",
			mutate: func(m *GoModuleV3) {
				m.Version = "v2.0.0"
			},
		},
		{
			name: "unexpected artifact",
			mutate: func(m *GoModuleV3) {
				m.Artifacts[0].Path = "../evil.zip"
			},
		},
		{
			name: "duplicate artifact",
			mutate: func(m *GoModuleV3) {
				m.Artifacts[1].Path = "module-0001.zip"
			},
		},
		{
			name: "missing artifact",
			mutate: func(m *GoModuleV3) {
				m.Artifacts = m.Artifacts[:2]
			},
		},
		{
			name: "negative size",
			mutate: func(m *GoModuleV3) {
				m.Artifacts[0].Size = -1
			},
		},
		{
			name: "exceeds size limit",
			mutate: func(m *GoModuleV3) {
				m.Artifacts[0].Size = MaxGoModuleTotalSizeV3 + 1
			},
		},
		{
			name: "invalid digest",
			mutate: func(m *GoModuleV3) {
				m.Artifacts[0].SHA256 = "sha256:invalid"
			},
		},
		{
			name: "invalid digest hex",
			mutate: func(m *GoModuleV3) {
				m.Artifacts[0].SHA256 = "sha256:" + strings.Repeat("z", 64)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mod := validGoModuleV3()
			tt.mutate(&mod)

			if err := ValidateGoModulesV3([]GoModuleV3{mod}); err == nil {
				t.Fatal("invalid module metadata accepted")
			}
		})
	}
}

func TestValidateGoModulesV3RejectsTooManyModules(t *testing.T) {
	mod := validGoModuleV3()

	if err := ValidateGoModulesV3([]GoModuleV3{mod, mod}); err == nil {
		t.Fatal("expected module count limit error")
	}
}
