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

	if _, err := ParseGoModuleGraphV3(moduleGraphFixture(17)); err == nil {
		t.Fatal("17-module graph unexpectedly accepted")
	}
}
