package capsule

import "testing"

func TestCompareGoModuleGraphV3(t *testing.T) {
	expected := []GoModuleV3{
		{Path: "example.com/a", Version: "v1.0.0"},
		{Path: "example.com/b", Version: "v1.0.0"},
	}

	tests := []struct {
		name    string
		actual  []GoModuleIdentityV3
		wantErr bool
	}{
		{
			name: "exact match",
			actual: []GoModuleIdentityV3{
				{Path: "example.com/a", Version: "v1.0.0"},
				{Path: "example.com/b", Version: "v1.0.0"},
			},
		},
		{
			name: "missing module",
			actual: []GoModuleIdentityV3{
				{Path: "example.com/a", Version: "v1.0.0"},
			},
			wantErr: true,
		},
		{
			name: "extra module",
			actual: []GoModuleIdentityV3{
				{Path: "example.com/a", Version: "v1.0.0"},
				{Path: "example.com/b", Version: "v1.0.0"},
				{Path: "example.com/c", Version: "v1.0.0"},
			},
			wantErr: true,
		},
		{
			name: "wrong version",
			actual: []GoModuleIdentityV3{
				{Path: "example.com/a", Version: "v1.0.0"},
				{Path: "example.com/b", Version: "v1.1.0"},
			},
			wantErr: true,
		},
		{
			name: "duplicate module",
			actual: []GoModuleIdentityV3{
				{Path: "example.com/a", Version: "v1.0.0"},
				{Path: "example.com/a", Version: "v1.0.0"},
			},
			wantErr: true,
		},
		{
			name: "unsorted graph",
			actual: []GoModuleIdentityV3{
				{Path: "example.com/b", Version: "v1.0.0"},
				{Path: "example.com/a", Version: "v1.0.0"},
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := CompareGoModuleGraphV3(expected, tc.actual)
			if tc.wantErr && err == nil {
				t.Fatal("invalid graph accepted")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("valid graph rejected: %v", err)
			}
		})
	}
}
