package docker

import (
	"reflect"
	"testing"
)

func TestSelectiveGoModuleDownloadArgsV3(t *testing.T) {
	expected := []ExpectedGoModule{
		{Path: "example.com/a", Version: "v1.0.0"},
		{Path: "example.com/b", Version: "v1.0.0"},
		{Path: "example.com/c", Version: "v1.0.0"},
	}

	materialized := []ExpectedGoModule{
		{Path: "example.com/a", Version: "v1.0.0"},
		{Path: "example.com/c", Version: "v1.0.0"},
	}

	args, err := selectiveGoModuleDownloadArgsV3(
		expected, materialized,
	)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{
		"go", "mod", "download",
		"example.com/a@v1.0.0",
		"example.com/c@v1.0.0",
	}

	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %q; want %q", args, want)
	}
}

func TestSelectiveGoModuleDownloadArgsV3RejectsInvalid(t *testing.T) {
	expected := []ExpectedGoModule{
		{Path: "example.com/a", Version: "v1.0.0"},
	}

	tests := []struct {
		name    string
		modules []ExpectedGoModule
	}{
		{
			name: "unexpected module",
			modules: []ExpectedGoModule{
				{Path: "example.com/b", Version: "v1.0.0"},
			},
		},
		{
			name: "wrong version",
			modules: []ExpectedGoModule{
				{Path: "example.com/a", Version: "v2.0.0"},
			},
		},
		{
			name: "duplicate",
			modules: []ExpectedGoModule{
				{Path: "example.com/a", Version: "v1.0.0"},
				{Path: "example.com/a", Version: "v1.0.0"},
			},
		},
		{
			name: "invalid path",
			modules: []ExpectedGoModule{
				{Path: "../escape", Version: "v1.0.0"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := selectiveGoModuleDownloadArgsV3(
				expected, tt.modules,
			); err == nil {
				t.Fatal("invalid module selection accepted")
			}
		})
	}
}

func TestSelectiveGoModuleDownloadArgsV3Empty(t *testing.T) {
	args, err := selectiveGoModuleDownloadArgsV3(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if args != nil {
		t.Fatalf("empty graph produced download command: %q", args)
	}
}
