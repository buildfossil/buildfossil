package main

import "testing"

func TestSelectRewVersion(t *testing.T) {
	tests := []struct {
		name     string
		injected string
		module   string
		want     string
	}{
		{
			name:   "local development",
			module: "(devel)",
			want:   "dev",
		},
		{
			name: "empty metadata",
			want: "dev",
		},
		{
			name:   "module release",
			module: "v0.1.0-alpha.1",
			want:   "v0.1.0-alpha.1",
		},
		{
			name:   "module pseudo-version",
			module: "v0.0.0-20261011022057-d3972c5a099b",
			want:   "v0.0.0-20261011022057-d3972c5a099b",
		},
		{
			name:   "dirty module",
			module: "v0.0.0-20261011032540-5ea24498992d+dirty",
			want:   "dev",
		},
		{
			name:     "injected release",
			injected: "v0.1.0-alpha.1",
			module:   "(devel)",
			want:     "v0.1.0-alpha.1",
		},
		{
			name:     "injected version has priority",
			injected: "v0.2.0-alpha.1",
			module:   "v0.1.0-alpha.1",
			want:     "v0.2.0-alpha.1",
		},
		{
			name:     "trim injected whitespace",
			injected: " v0.1.0-alpha.1 ",
			want:     "v0.1.0-alpha.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selectRewVersion(tt.injected, tt.module)
			if got != tt.want {
				t.Fatalf("version = %q, want %q", got, tt.want)
			}
		})
	}
}
