package capsule

import "testing"

func TestGoModulesV3ZeroDependencyContract(t *testing.T) {
	t.Run("zero modules accepted", func(t *testing.T) {
		if err := ValidateGoModulesV3(nil); err != nil {
			t.Fatalf("zero Go modules rejected: %v", err)
		}
	})

	t.Run("one module remains supported", func(t *testing.T) {
		modules := []GoModuleV3{
			{
				Path:    "example.com/fixture",
				Version: "v1.0.0",
				Artifacts: []GoModuleArtifact{
					{
						Path:   "module-0001.zip",
						Size:   1,
						SHA256: SHA256([]byte("a")),
					},
					{
						Path:   "module-0001.mod",
						Size:   1,
						SHA256: SHA256([]byte("b")),
					},
					{
						Path:   "module-0001.info",
						Size:   1,
						SHA256: SHA256([]byte("c")),
					},
				},
			},
		}

		if err := ValidateGoModulesV3(modules); err != nil {
			t.Fatalf("one Go module rejected: %v", err)
		}
	})

	t.Run("multiple modules rejected", func(t *testing.T) {
		modules := []GoModuleV3{
			{Path: "example.com/a", Version: "v1.0.0"},
			{Path: "example.com/b", Version: "v1.0.0"},
		}

		if err := ValidateGoModulesV3(modules); err == nil {
			t.Fatal("expected multiple modules to be rejected")
		}
	})
}
