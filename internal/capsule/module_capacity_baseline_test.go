package capsule

import (
	"fmt"
	"io"
	"strings"
	"testing"
)

func moduleGraphFixture(count int) []byte {
	var b strings.Builder
	b.WriteString("{\"Path\":\"example.com/app\",\"Main\":true}\n")

	for i := 0; i < count; i++ {
		fmt.Fprintf(
			&b,
			"{\"Path\":\"example.com/mod%04d\",\"Version\":\"v1.0.0\"}\n",
			i,
		)
	}

	return []byte(b.String())
}

func TestGoModuleGraphCapacityBoundaries(t *testing.T) {
	for _, count := range []int{0, 1, 8, 9, 16, 17, 64, 65} {
		t.Run(fmt.Sprintf("modules_%d", count), func(t *testing.T) {
			modules, err := ParseGoModuleGraphV3(
				moduleGraphFixture(count),
			)

			if count > MaxGoSelectedModulesV3 {
				if err == nil {
					t.Fatal("expected module limit rejection")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(modules) != count {
				t.Fatalf("got %d modules, want %d", len(modules), count)
			}
		})
	}
}

func BenchmarkParseGoModuleGraphV3(b *testing.B) {
	for _, count := range []int{0, 1, 8, 9, 16, 17, 64, 65} {
		b.Run(fmt.Sprintf("modules_%d", count), func(b *testing.B) {
			input := moduleGraphFixture(count)
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, err := ParseGoModuleGraphV3(input)

				if count > MaxGoSelectedModulesV3 {
					if err == nil {
						b.Fatal("expected module limit rejection")
					}
				} else if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func TestGoModuleMetadataCapacityBoundaries(t *testing.T) {
	for _, count := range []int{0, 1, 8, 9, 16, 17} {
		t.Run(fmt.Sprintf("modules_%d", count), func(t *testing.T) {
			modules := make([]GoModuleV3, 0, count)

			for i := 0; i < count; i++ {
				modules = append(modules, multiModuleFixture(
					i+1,
					fmt.Sprintf("example.com/mod%04d", i),
				))
			}

			err := ValidateGoModulesV3(modules)

			if count > MaxGoModulesV3 {
				if err == nil {
					t.Fatal("expected module limit rejection")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected validation error: %v", err)
			}
		})
	}
}

func BenchmarkWriteWorkspaceV3(b *testing.B) {
	manifest, files, artifacts := makeWorkspaceV3Fixture(b)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if err := WriteWorkspaceV3(
			io.Discard,
			manifest,
			files,
			artifacts,
		); err != nil {
			b.Fatal(err)
		}
	}
}
