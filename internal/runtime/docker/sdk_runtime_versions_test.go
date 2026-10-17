package docker

import "testing"

func TestSelectReplayImageForExactGoVersions(t *testing.T) {
	tests := []struct {
		version string
		want    string
		ok      bool
	}{
		{"1.26.6", goReplayImage, true},
		{"1.25.14", goReplayImage125, true},
		{"1.26.5", "", false},
		{"1.25.13", "", false},
		{"1.25.0", "", false},
		{"1.27.0", "", false},
		{"", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			got, err := selectReplayImage(&RuntimeSpec{
				Kind:    "go",
				Version: tt.version,
			})

			if (err == nil) != tt.ok {
				t.Fatalf("version %q: unexpected error %v", tt.version, err)
			}
			if got != tt.want {
				t.Fatalf("version %q: image = %q, want %q",
					tt.version, got, tt.want)
			}
		})
	}
}

func TestSelectReplayImageRejectsUnknownKind(t *testing.T) {
	if _, err := selectReplayImage(&RuntimeSpec{
		Kind:    "python",
		Version: "1.25.14",
	}); err == nil {
		t.Fatal("unsupported runtime kind accepted")
	}
}
