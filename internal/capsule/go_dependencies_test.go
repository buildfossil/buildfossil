package capsule

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInspectGoModuleRequirements(t *testing.T) {
	tests := []struct {
		name         string
		goMod        string
		wantRequires bool
		wantErr      bool
	}{
		{
			name:  "no dependencies",
			goMod: "module example.com/demo\n\ngo 1.26.0\n",
		},
		{
			name: "external dependency",
			goMod: `module example.com/demo

go 1.26.0

require github.com/google/uuid v1.6.0
`,
			wantRequires: true,
		},
		{
			name: "local replacement",
			goMod: `module example.com/demo

go 1.26.0

require example.com/local v1.0.0

replace example.com/local => ./local
`,
			wantRequires: true,
		},
		{
			name:    "invalid go.mod",
			goMod:   "module\nrequire invalid\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workspace := t.TempDir()

			if err := os.WriteFile(
				filepath.Join(workspace, "go.mod"),
				[]byte(tt.goMod),
				0600,
			); err != nil {
				t.Fatal(err)
			}

			got, err := InspectGoModuleRequirements(workspace)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected parsing error")
				}
				return
			}

			if err != nil {
				t.Fatalf("inspect dependencies: %v", err)
			}

			if got != tt.wantRequires {
				t.Fatalf(
					"requires = %v, want %v",
					got,
					tt.wantRequires,
				)
			}
		})
	}
}
