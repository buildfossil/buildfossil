package capsule

import "testing"

func TestRuntimeValidate(t *testing.T) {
	tests := []struct {
		name    string
		runtime Runtime
		wantErr bool
	}{
		{
			name: "supported Go",
			runtime: Runtime{
				Kind:    "go",
				Version: SupportedGoVersion,
			},
		},
		{
			name: "unknown kind",
			runtime: Runtime{
				Kind:    "python",
				Version: SupportedGoVersion,
			},
			wantErr: true,
		},
		{
			name: "unsupported Go version",
			runtime: Runtime{
				Kind:    "go",
				Version: "1.25.0",
			},
			wantErr: true,
		},
		{
			name: "empty version",
			runtime: Runtime{
				Kind: "go",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.runtime.Validate()

			if (err != nil) != tt.wantErr {
				t.Fatalf(
					"Validate() error = %v; wantErr = %t",
					err,
					tt.wantErr,
				)
			}
		})
	}
}
