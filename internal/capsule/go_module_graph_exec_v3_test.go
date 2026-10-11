package capsule

import "testing"

func TestListGoModuleGraphV3RejectsEmptyPaths(t *testing.T) {
	if _, err := ListGoModuleGraphV3("", "/tmp/cache"); err == nil {
		t.Fatal("empty workspace accepted")
	}

	if _, err := ListGoModuleGraphV3("/tmp/workspace", ""); err == nil {
		t.Fatal("empty module cache accepted")
	}
}
