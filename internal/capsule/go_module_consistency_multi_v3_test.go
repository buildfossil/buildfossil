package capsule

import "testing"

func TestGoModuleConsistencyV3MultiModule(t *testing.T) {
	modules := []GoModuleV3{
		multiModuleFixture(1, "example.com/a"),
		multiModuleFixture(2, "example.com/b"),
		multiModuleFixture(3, "example.com/transitive"),
	}

	tests := []struct {
		name    string
		goMod   string
		modules []GoModuleV3
		wantErr bool
	}{
		{
			name: "direct plus transitive dependencies",
			goMod: `module example.com/app

go 1.26.6

require (
	example.com/a v1.0.0
	example.com/b v1.0.0
)
`,
			modules: modules,
		},
		{
			name: "missing required dependency",
			goMod: `module example.com/app

go 1.26.6

require example.com/missing v1.0.0
`,
			modules: modules,
			wantErr: true,
		},
		{
			name: "wrong required version",
			goMod: `module example.com/app

go 1.26.6

require example.com/a v1.2.0
`,
			modules: modules,
			wantErr: true,
		},
		{
			name: "indirect requirement is valid",
			goMod: `module example.com/app

go 1.26.6

require example.com/transitive v1.0.0 // indirect
`,
			modules: modules,
		},
		{
			name: "replacement rejected",
			goMod: `module example.com/app

go 1.26.6

require example.com/a v1.0.0
replace example.com/a => ../a
`,
			modules: modules,
			wantErr: true,
		},
		{
			name: "empty dependency graph",
			goMod: `module example.com/app

go 1.26.6
`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateGoModuleConsistencyV3(
				[]byte(tc.goMod),
				tc.modules,
			)

			if tc.wantErr && err == nil {
				t.Fatal("invalid dependency declaration accepted")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("valid dependency declaration rejected: %v", err)
			}
		})
	}
}
