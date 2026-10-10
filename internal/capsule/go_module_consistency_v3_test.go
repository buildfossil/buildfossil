package capsule

import "testing"

func TestValidateGoModuleConsistencyV3(t *testing.T) {
	const dependency = "example.com/fixture"
	const version = "v1.0.0"

	one := []GoModuleV3{{
		Path:    dependency,
		Version: version,
		Artifacts: []GoModuleArtifact{
			{Path: "module-0001.zip", Size: 1, SHA256: SHA256([]byte("a"))},
			{Path: "module-0001.mod", Size: 1, SHA256: SHA256([]byte("b"))},
			{Path: "module-0001.info", Size: 1, SHA256: SHA256([]byte("c"))},
		},
	}}

	tests := []struct {
		name    string
		goMod   string
		modules []GoModuleV3
		wantErr bool
	}{
		{
			name:  "zero modules match",
			goMod: "module example.com/project\n\ngo 1.26.6\n",
		},
		{
			name:    "one module matches",
			goMod:   "module example.com/project\n\ngo 1.26.6\nrequire example.com/fixture v1.0.0\n",
			modules: one,
		},
		{
			name:    "missing manifest dependency",
			goMod:   "module example.com/project\n\ngo 1.26.6\nrequire example.com/fixture v1.0.0\n",
			wantErr: true,
		},
		{
			name:    "unexpected manifest dependency",
			goMod:   "module example.com/project\n\ngo 1.26.6\n",
			modules: one,
			wantErr: true,
		},
		{
			name:    "wrong version",
			goMod:   "module example.com/project\n\ngo 1.26.6\nrequire example.com/fixture v1.0.1\n",
			modules: one,
			wantErr: true,
		},
		{
			name:    "indirect dependency",
			goMod:   "module example.com/project\n\ngo 1.26.6\nrequire example.com/fixture v1.0.0 // indirect\n",
			modules: one,
			wantErr: true,
		},
		{
			name:    "unsupported replace",
			goMod:   "module example.com/project\n\ngo 1.26.6\nreplace example.com/fixture => ../fixture\n",
			wantErr: true,
		},
		{
			name:    "multiple dependencies",
			goMod:   "module example.com/project\n\ngo 1.26.6\nrequire (\nexample.com/a v1.0.0\nexample.com/b v1.0.0\n)\n",
			wantErr: true,
		},
		{
			name:    "missing module declaration",
			goMod:   "go 1.26.6\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateGoModuleConsistencyV3(
				[]byte(tt.goMod),
				tt.modules,
			)
			if tt.wantErr && err == nil {
				t.Fatal("expected rejection")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected rejection: %v", err)
			}
		})
	}
}
