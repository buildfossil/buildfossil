package capsule

import (
	"strings"
	"testing"
)

func TestReadGoDependenciesForCaptureV3(t *testing.T) {
	t.Run("zero dependencies", func(t *testing.T) {
		goMod := []byte("module example.com/zero\n\ngo 1.26.6\n")

		modules, artifacts, err := ReadGoDependenciesForCaptureV3(
			goMod,
			nil,
			"",
		)
		if err != nil {
			t.Fatal(err)
		}

		if len(modules) != 0 || len(artifacts) != 0 {
			t.Fatalf(
				"expected no modules or artifacts, got %d modules, %d artifacts",
				len(modules),
				len(artifacts),
			)
		}
	})

	t.Run("reject replace without dependencies", func(t *testing.T) {
		goMod := []byte(
			"module example.com/zero\n\ngo 1.26.6\n" +
				"replace example.com/other => ../other\n",
		)

		if _, _, err := ReadGoDependenciesForCaptureV3(
			goMod, nil, "",
		); err == nil {
			t.Fatal("expected replace directive to be rejected")
		}
	})

	t.Run("reject multiple dependencies", func(t *testing.T) {
		goMod := []byte(
			"module example.com/multi\n\ngo 1.26.6\n" +
				"require (\n" +
				"example.com/a v1.0.0\n" +
				"example.com/b v1.0.0\n" +
				")\n",
		)

		_, _, err := ReadGoDependenciesForCaptureV3(
			goMod, nil, "",
		)
		if err == nil || !strings.Contains(err.Error(), "at most one") {
			t.Fatalf("expected dependency limit error, got %v", err)
		}
	})

	t.Run("reject indirect dependency", func(t *testing.T) {
		goMod := []byte(
			"module example.com/indirect\n\ngo 1.26.6\n" +
				"require example.com/a v1.0.0 // indirect\n",
		)

		if _, _, err := ReadGoDependenciesForCaptureV3(
			goMod, nil, "",
		); err == nil {
			t.Fatal("expected indirect dependency to be rejected")
		}
	})
}
