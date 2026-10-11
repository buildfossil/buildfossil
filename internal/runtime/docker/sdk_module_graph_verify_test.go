package docker

import "testing"

func TestCompareDockerGoModuleGraph(t *testing.T) {
	expected := []ExpectedGoModule{
		{Path: "example.com/a", Version: "v1.0.0"},
		{Path: "example.com/b", Version: "v1.0.0"},
	}

	tests := []struct {
		name    string
		actual  []ExpectedGoModule
		wantErr bool
	}{
		{
			name:   "exact match",
			actual: expected,
		},
		{
			name: "missing module",
			actual: []ExpectedGoModule{
				{Path: "example.com/a", Version: "v1.0.0"},
			},
			wantErr: true,
		},
		{
			name: "wrong version",
			actual: []ExpectedGoModule{
				{Path: "example.com/a", Version: "v1.0.0"},
				{Path: "example.com/b", Version: "v1.1.0"},
			},
			wantErr: true,
		},
		{
			name: "extra module",
			actual: append(
				append([]ExpectedGoModule{}, expected...),
				ExpectedGoModule{Path: "example.com/c", Version: "v1.0.0"},
			),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := compareDockerGoModuleGraph(expected, tc.actual)
			if (err != nil) != tc.wantErr {
				t.Fatalf("error = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}
