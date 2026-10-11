package capsule

import (
	"strings"
	"testing"
)

func TestSelectGoBuildModulesV3(t *testing.T) {
	selected := []GoModuleIdentityV3{
		{Path: "example.com/a", Version: "v1.0.0"},
		{Path: "example.com/b", Version: "v1.2.0"},
		{Path: "example.com/unused", Version: "v1.0.0"},
	}

	const valid = `
{"Standard":true}
{"Module":{"Path":"example.com/root","Main":true}}
{"Module":{"Path":"example.com/b","Version":"v1.2.0"}}
{"Module":{"Path":"example.com/a","Version":"v1.0.0"}}
{"Module":{"Path":"example.com/a","Version":"v1.0.0"}}
`

	t.Run("select build modules", func(t *testing.T) {
		got, err := SelectGoBuildModulesV3(
			strings.NewReader(valid), selected,
		)
		if err != nil {
			t.Fatal(err)
		}

		if len(got) != 2 ||
			got[0].Path != "example.com/a" ||
			got[1].Path != "example.com/b" {
			t.Fatalf("unexpected selected modules: %+v", got)
		}
	})

	tests := []struct {
		name string
		data string
	}{
		{
			name: "empty package stream",
			data: "",
		},
		{
			name: "incomplete package",
			data: `{"Incomplete":true,"Module":{"Main":true,"Path":"example.com/root"}}`,
		},
		{
			name: "package error",
			data: `{"Error":{"Err":"missing import"}}`,
		},
		{
			name: "dependency error",
			data: `{"DepsErrors":[{"Err":"module not found"}]}`,
		},
		{
			name: "external module missing from graph",
			data: `{"Module":{"Path":"example.com/missing","Version":"v1.0.0"}}`,
		},
		{
			name: "wrong selected version",
			data: `{"Module":{"Path":"example.com/a","Version":"v9.0.0"}}`,
		},
		{
			name: "missing module metadata",
			data: `{"Standard":false}`,
		},
		{
			name: "invalid module identity",
			data: `{"Module":{"Path":"INVALID","Version":"v1.0.0"}}`,
		},
		{
			name: "malformed json",
			data: `{"Module":`,
		},
		{
			name: "standard package with module",
			data: `{"Standard":true,"Module":{"Path":"example.com/a","Version":"v1.0.0"}}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := SelectGoBuildModulesV3(
				strings.NewReader(tc.data), selected,
			); err == nil {
				t.Fatal("invalid dependency selection accepted")
			}
		})
	}
}

func TestSelectGoBuildModulesV3ZeroDependencies(t *testing.T) {
	const packages = `
{"Standard":true}
{"Module":{"Path":"example.com/root","Main":true}}
`

	got, err := SelectGoBuildModulesV3(
		strings.NewReader(packages), nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("expected zero modules, got %+v", got)
	}
}
