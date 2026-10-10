package capsule

import "testing"

func TestGoDependencyStatusValidate(t *testing.T) {
	tests := []struct {
		name    string
		status  GoDependencyStatus
		wantErr bool
	}{
		{
			name: "ready",
			status: GoDependencyStatus{
				State: GoDependenciesReady,
			},
		},
		{
			name: "unavailable with reason",
			status: GoDependencyStatus{
				State:  GoDependenciesUnavailable,
				Reason: "dependency unavailable in offline environment",
			},
		},
		{
			name: "unavailable without reason",
			status: GoDependencyStatus{
				State: GoDependenciesUnavailable,
			},
			wantErr: true,
		},
		{
			name: "unknown",
			status: GoDependencyStatus{
				State: GoDependenciesUnknown,
			},
		},
		{
			name: "invalid state",
			status: GoDependencyStatus{
				State: "invalid",
			},
			wantErr: true,
		},
		{
			name:    "empty state",
			status:  GoDependencyStatus{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.status.Validate()

			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}
