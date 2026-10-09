package capsule

import "testing"

func TestManifestValidate(t *testing.T) {
	valid := func() Manifest {
		return Manifest{
			SchemaVersion: SchemaVersion,
			Execution: Execution{
				Argv:       []string{"go", "test", "./..."},
				WorkingDir: ".",
				ExitCode:   1,
			},
			Platform: Platform{
				OS:           "linux",
				Architecture: "amd64",
			},
		}
	}

	tests := []struct {
		name    string
		modify  func(*Manifest)
		wantErr bool
	}{
		{
			name: "valid manifest",
		},
		{
			name: "unsupported schema",
			modify: func(m *Manifest) {
				m.SchemaVersion = 99
			},
			wantErr: true,
		},
		{
			name: "missing command",
			modify: func(m *Manifest) {
				m.Execution.Argv = nil
			},
			wantErr: true,
		},
		{
			name: "empty executable",
			modify: func(m *Manifest) {
				m.Execution.Argv[0] = ""
			},
			wantErr: true,
		},
		{
			name: "missing platform",
			modify: func(m *Manifest) {
				m.Platform.OS = ""
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := valid()

			if tt.modify != nil {
				tt.modify(&m)
			}

			err := m.Validate()

			if (err != nil) != tt.wantErr {
				t.Errorf(
					"Validate() error = %v, wantErr = %v",
					err,
					tt.wantErr,
				)
			}
		})
	}
}
