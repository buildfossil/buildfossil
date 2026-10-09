package capsule

import "testing"

func TestSHA256(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "known SHA256 vector",
			input: "abc",
			want:  "sha256:ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
		},
		{
			name:  "empty input",
			input: "",
			want:  "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SHA256([]byte(tt.input))

			if got != tt.want {
				t.Errorf("SHA256() = %q, want %q", got, tt.want)
			}
		})
	}
}
