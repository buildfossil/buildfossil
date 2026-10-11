package replay

import "testing"

func TestParseReplayMemoryV3(t *testing.T) {
	tests := []struct {
		value string
		want  int64
		valid bool
	}{
		{"256m", 256 << 20, true},
		{"512m", 512 << 20, true},
		{"1g", 1 << 30, true},
		{"2g", 2 << 30, true},
		{"", 0, false},
		{"0", 0, false},
		{"128m", 0, false},
		{"3g", 0, false},
		{"1G", 0, false},
		{"unlimited", 0, false},
		{"-1", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			got, err := ParseReplayMemoryV3(tt.value)

			if (err == nil) != tt.valid {
				t.Fatalf("unexpected validation result: %v", err)
			}

			if tt.valid && got != tt.want {
				t.Fatalf("memory = %d; want %d", got, tt.want)
			}
		})
	}
}
