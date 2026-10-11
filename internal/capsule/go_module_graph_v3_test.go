package capsule

import (
	"fmt"
	"strings"
	"testing"
)

func TestParseGoModuleGraphV3(t *testing.T) {
	input := `
{"Path":"example.com/app","Main":true}
{"Path":"example.com/z","Version":"v1.0.0"}
{"Path":"example.com/a","Version":"v1.2.0","Indirect":true}
`

	modules, err := ParseGoModuleGraphV3([]byte(input))
	if err != nil {
		t.Fatal(err)
	}

	if len(modules) != 2 {
		t.Fatalf("expected 2 modules, got %d", len(modules))
	}

	if modules[0].Path != "example.com/a" ||
		modules[1].Path != "example.com/z" {
		t.Fatalf("modules not sorted: %+v", modules)
	}
}

func TestParseGoModuleGraphV3RejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"malformed JSON", "{"},
		{"missing main", `{"Path":"example.com/a","Version":"v1.0.0"}`},
		{"duplicate main", `{"Path":"example.com/app","Main":true}
{"Path":"example.com/app","Main":true}`},
		{"duplicate dependency", `{"Path":"example.com/app","Main":true}
{"Path":"example.com/a","Version":"v1.0.0"}
{"Path":"example.com/a","Version":"v1.1.0"}`},
		{"replacement", `{"Path":"example.com/app","Main":true}
{"Path":"example.com/a","Version":"v1.0.0","Replace":{"Path":"example.com/b","Version":"v1.0.0"}}`},
		{"module error", `{"Path":"example.com/app","Main":true}
{"Path":"example.com/a","Version":"v1.0.0","Error":{"Err":"failed"}}`},
		{"invalid version", `{"Path":"example.com/app","Main":true}
{"Path":"example.com/a","Version":"latest"}`},
		{"unexpected field", `{"Path":"example.com/app","Main":true,"Unexpected":true}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseGoModuleGraphV3([]byte(tc.input)); err == nil {
				t.Fatal("invalid module graph accepted")
			}
		})
	}
}

func TestParseGoModuleGraphV3SeventeenModules(t *testing.T) {
	var input strings.Builder
	input.WriteString(`{"Path":"example.com/root","Main":true}` + "\n")

	for i := 0; i < 17; i++ {
		fmt.Fprintf(
			&input,
			"{\"Path\":\"example.com/mod%02d\",\"Version\":\"v1.0.0\"}\n",
			i,
		)
	}

	modules, err := ParseGoModuleGraphV3([]byte(input.String()))
	if err != nil {
		t.Fatalf("17-module graph rejected: %v", err)
	}

	if len(modules) != 17 {
		t.Fatalf("module count = %d, want 17", len(modules))
	}
}

func TestParseGoModuleGraphV3RejectsTooManyModules(t *testing.T) {
	var input strings.Builder
	input.WriteString(`{"Path":"example.com/app","Main":true}` + "\n")

	for i := 0; i < MaxGoSelectedModulesV3+1; i++ {
		fmt.Fprintf(
			&input,
			"{\"Path\":\"example.com/mod%02d\",\"Version\":\"v1.0.0\"}\n",
			i,
		)
	}

	if _, err := ParseGoModuleGraphV3([]byte(input.String())); err == nil {
		t.Fatal("oversized module graph accepted")
	}
}

func TestParseGoModuleGraphV3WithGoMetadata(t *testing.T) {
	input := `
{"Path":"github.com/spf13/cobra","Main":true,"GoVersion":"1.15"}
{"Path":"github.com/spf13/pflag","Version":"v1.0.9","Dir":"/tmp/pflag","GoMod":"/tmp/pflag.mod","GoVersion":"1.12","Sum":"h1:test","GoModSum":"h1:test"}
{"Path":"github.com/russross/blackfriday/v2","Version":"v2.1.0","Indirect":true,"Time":"2020-10-27T03:47:54Z"}
`

	modules, err := ParseGoModuleGraphV3([]byte(input))
	if err != nil {
		t.Fatalf("realistic Go module JSON rejected: %v", err)
	}

	if len(modules) != 2 {
		t.Fatalf("expected 2 modules, got %d", len(modules))
	}

	if modules[0].Path != "github.com/russross/blackfriday/v2" ||
		modules[1].Path != "github.com/spf13/pflag" {
		t.Fatalf("unexpected module order: %+v", modules)
	}
}
