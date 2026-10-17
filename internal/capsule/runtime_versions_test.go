package capsule

import "testing"

func TestSupportedGoVersionPolicy(t *testing.T) {
	tests := []struct {
		version string
		want    bool
	}{
		{"1.26.6", true},
		{"1.25.14", true},
		{"1.26.5", false},
		{"1.25.13", false},
		{"1.25.0", false},
		{"1.27.0", false},
		{"", false},
		{"go1.25.14", false},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			if got := IsSupportedGoVersion(tt.version); got != tt.want {
				t.Fatalf(
					"IsSupportedGoVersion(%q) = %v, want %v",
					tt.version, got, tt.want,
				)
			}

			err := (Runtime{
				Kind:    "go",
				Version: tt.version,
			}).Validate()

			if (err == nil) != tt.want {
				t.Fatalf(
					"Runtime.Validate(%q) error = %v, want valid = %v",
					tt.version, err, tt.want,
				)
			}
		})
	}
}

func TestRuntimeRejectsUnsupportedKind(t *testing.T) {
	if err := (Runtime{
		Kind:    "python",
		Version: SupportedGoVersion,
	}).Validate(); err == nil {
		t.Fatal("unsupported runtime kind accepted")
	}
}
