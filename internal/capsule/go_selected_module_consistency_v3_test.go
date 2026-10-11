package capsule

import (
	"strings"
	"testing"
)

func TestGoSelectedModuleConsistencyV3(t *testing.T) {
	const rootMod = `module example.com/root

go 1.26.6

require (
    example.com/a v1.0.0
    example.com/b v1.2.0 // indirect
)
`

	makeManifest := func() Manifest {
		return Manifest{
			GoSelectedModules: []GoModuleIdentityV3{
				{Path: "example.com/a", Version: "v1.0.0"},
				{Path: "example.com/b", Version: "v1.2.0"},
				{Path: "example.com/extra", Version: "v1.0.0"},
			},
		}
	}

	t.Run("all root requirements present", func(t *testing.T) {
		m := makeManifest()
		if err := ValidateGoSelectedModuleConsistencyV3(
			[]byte(rootMod), m,
		); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("missing required module", func(t *testing.T) {
		m := makeManifest()
		m.GoSelectedModules = m.GoSelectedModules[1:]

		if err := ValidateGoSelectedModuleConsistencyV3(
			[]byte(rootMod), m,
		); err == nil {
			t.Fatal("missing required module accepted")
		}
	})

	t.Run("mismatched version", func(t *testing.T) {
		m := makeManifest()
		m.GoSelectedModules[0].Version = "v1.9.0"

		if err := ValidateGoSelectedModuleConsistencyV3(
			[]byte(rootMod), m,
		); err == nil {
			t.Fatal("wrong selected version accepted")
		}
	})

	t.Run("reject replace", func(t *testing.T) {
		m := makeManifest()
		replaced := rootMod +
			"\nreplace example.com/a => example.com/c v1.0.0\n"

		if err := ValidateGoSelectedModuleConsistencyV3(
			[]byte(replaced), m,
		); err == nil {
			t.Fatal("replacement accepted")
		}
	})

	t.Run("reject exclude", func(t *testing.T) {
		m := makeManifest()
		excluded := rootMod +
			"\nexclude example.com/a v1.0.0\n"

		if err := ValidateGoSelectedModuleConsistencyV3(
			[]byte(excluded), m,
		); err == nil {
			t.Fatal("exclude accepted")
		}
	})

	t.Run("dependency-free root", func(t *testing.T) {
		m := Manifest{
			GoSelectedModules: []GoModuleIdentityV3{},
		}

		err := ValidateGoSelectedModuleConsistencyV3(
			[]byte("module example.com/root\n\ngo 1.26.6\n"),
			m,
		)

		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("legacy fallback", func(t *testing.T) {
		m := Manifest{}
		err := ValidateGoSelectedModuleConsistencyV3(
			[]byte("module example.com/root\n\ngo 1.26.6\n"),
			m,
		)
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("reject selected modules for zero requirements", func(t *testing.T) {
		m := makeManifest()
		err := ValidateGoSelectedModuleConsistencyV3(
			[]byte("module example.com/root\n\ngo 1.26.6\n"),
			m,
		)
		if err == nil {
			t.Fatal("extraneous graph accepted")
		}
	})

	t.Run("invalid go.mod", func(t *testing.T) {
		m := makeManifest()
		if err := ValidateGoSelectedModuleConsistencyV3(
			[]byte(strings.Repeat("garbage\n", 3)), m,
		); err == nil {
			t.Fatal("invalid go.mod accepted")
		}
	})
}
