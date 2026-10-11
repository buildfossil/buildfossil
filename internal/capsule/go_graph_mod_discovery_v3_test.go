package capsule

import (
	"strings"
	"testing"
)

func TestParseGoGraphModRequirementsV3(t *testing.T) {
	selected := []GoModuleIdentityV3{
		{Path: "github.com/mattn/go-colorable", Version: "v0.1.13"},
		{Path: "github.com/mattn/go-isatty", Version: "v0.0.19"},
		{Path: "golang.org/x/sys", Version: "v0.12.0"},
	}

	graph := []byte(strings.Join([]string{
		"github.com/mattn/go-colorable@v0.1.13 github.com/mattn/go-isatty@v0.0.16",
		"github.com/mattn/go-isatty@v0.0.16 golang.org/x/sys@v0.0.0-20220811171246-fbc7d0a398ab",
		"github.com/mattn/go-isatty@v0.0.19 golang.org/x/sys@v0.6.0",
		"github.com/mattn/go-colorable@v0.1.13 go@1.18",
		"github.com/mattn/go-isatty@v0.0.19 toolchain@go1.23",
	}, "\n") + "\n")

	got, err := ParseGoGraphModRequirementsV3(graph, "example.com/app", selected)
	if err != nil {
		t.Fatalf("parse graph: %v", err)
	}

	want := []GoModuleIdentityV3{
		{
			Path:    "github.com/mattn/go-isatty",
			Version: "v0.0.16",
		},
		{
			Path:    "golang.org/x/sys",
			Version: "v0.0.0-20220811171246-fbc7d0a398ab",
		},
		{
			Path:    "golang.org/x/sys",
			Version: "v0.6.0",
		},
	}

	if len(got) != len(want) {
		t.Fatalf("got %d graph versions, want %d: %+v", len(got), len(want), got)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("graph version %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestParseGoGraphModRequirementsV3RejectsInvalid(t *testing.T) {
	selected := []GoModuleIdentityV3{
		{Path: "example.com/a", Version: "v1.0.0"},
	}

	cases := []struct {
		name  string
		graph string
	}{
		{"malformed line", "example.com/a@v1.0.0"},
		{"invalid module", "example.com/a@v1.0.0 ../evil@v1.0.0"},
		{"missing version", "example.com/a@v1.0.0 example.com/b"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseGoGraphModRequirementsV3(
				[]byte(tc.graph), "example.com/app", selected,
			); err == nil {
				t.Fatal("invalid graph accepted")
			}
		})
	}
}

func TestParseGoGraphModRequirementsV3MainModule(t *testing.T) {
	graph := []byte(
		"example.com/app example.com/dependency@v1.0.0\n" +
			"example.com/dependency@v1.0.0 example.com/old@v0.1.0\n",
	)

	selected := []GoModuleIdentityV3{
		{Path: "example.com/dependency", Version: "v1.0.0"},
	}

	got, err := ParseGoGraphModRequirementsV3(
		graph, "example.com/app", selected,
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 1 ||
		got[0].Path != "example.com/old" ||
		got[0].Version != "v0.1.0" {
		t.Fatalf("unexpected graph versions: %+v", got)
	}

	_, err = ParseGoGraphModRequirementsV3(
		[]byte("example.com/unknown example.com/dependency@v1.0.0\n"),
		"example.com/app",
		selected,
	)
	if err == nil {
		t.Fatal("unknown unversioned module accepted")
	}
}

func TestParseGoGraphModRequirementsV3RejectsInvalidToolchainTokens(t *testing.T) {
	selected := []GoModuleIdentityV3{
		{Path: "example.com/a", Version: "v1.0.0"},
	}

	cases := []string{
		"go@invalid example.com/a@v1.0.0",
		"toolchain@go1.23 example.com/a@v1.0.0",
		"example.com/a@v1.0.0 go@invalid",
		"example.com/a@v1.0.0 toolchain@invalid",
		"example.com/a@v1.0.0 go@1.23/evil",
	}

	for _, graph := range cases {
		t.Run(graph, func(t *testing.T) {
			_, err := ParseGoGraphModRequirementsV3(
				[]byte(graph),
				"example.com/app",
				selected,
			)
			if err == nil {
				t.Fatal("invalid graph token accepted")
			}
		})
	}
}

func TestParseGoGraphModRequirementsV3ValidGoVersions(t *testing.T) {
	selected := []GoModuleIdentityV3{
		{Path: "example.com/a", Version: "v1.0.0"},
	}

	cases := []string{
		"example.com/a@v1.0.0 go@1.23",
		"example.com/a@v1.0.0 go@1.23.4",
		"example.com/a@v1.0.0 go@1.24rc1",
		"example.com/a@v1.0.0 toolchain@go1.23.4",
	}

	for _, graph := range cases {
		t.Run(graph, func(t *testing.T) {
			got, err := ParseGoGraphModRequirementsV3(
				[]byte(graph),
				"example.com/app",
				selected,
			)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != 0 {
				t.Fatalf("unexpected graph modules: %+v", got)
			}
		})
	}
}

func TestParseGoGraphModRequirementsV3ServiceNodesOnBothSides(t *testing.T) {
	selected := []GoModuleIdentityV3{
		{Path: "example.com/dep", Version: "v1.0.0"},
	}

	graph := []byte(
		"example.com/app go@1.23\n" +
			"go@1.23 toolchain@go1.23\n" +
			"example.com/app example.com/dep@v1.0.0\n",
	)

	got, err := ParseGoGraphModRequirementsV3(
		graph, "example.com/app", selected,
	)
	if err != nil {
		t.Fatalf("valid Go graph rejected: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("unexpected additional modules: %+v", got)
	}

	for _, invalid := range []string{
		"go@invalid toolchain@go1.23",
		"go@1.23 toolchain@invalid",
		"go@1.23/evil toolchain@go1.23",
	} {
		if _, err := ParseGoGraphModRequirementsV3(
			[]byte(invalid),
			"example.com/app",
			selected,
		); err == nil {
			t.Fatalf("invalid service graph accepted: %q", invalid)
		}
	}
}
