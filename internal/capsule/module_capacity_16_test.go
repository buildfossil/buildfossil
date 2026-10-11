package capsule

import "testing"

func TestGoModuleGraphSupports16Modules(t *testing.T) {
	modules, err := ParseGoModuleGraphV3(moduleGraphFixture(16))
	if err != nil {
		t.Fatalf("16-module graph rejected: %v", err)
	}
	if len(modules) != 16 {
		t.Fatalf("got %d modules, want 16", len(modules))
	}

	for _, count := range []int{17, MaxGoSelectedModulesV3} {
		modules, err := ParseGoModuleGraphV3(moduleGraphFixture(count))
		if err != nil {
			t.Fatalf("%d-module graph rejected: %v", count, err)
		}
		if len(modules) != count {
			t.Fatalf("got %d modules, want %d", len(modules), count)
		}
	}

	if _, err := ParseGoModuleGraphV3(
		moduleGraphFixture(MaxGoSelectedModulesV3 + 1),
	); err == nil {
		t.Fatal("65-module graph unexpectedly accepted")
	}
}
