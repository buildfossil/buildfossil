package replay

import (
	"testing"

	"github.com/buildfossil/buildfossil/internal/capsule"
)

func TestResultGoDependenciesOptional(t *testing.T) {
	result := Result{
		OriginalExitCode: 1,
		ReplayExitCode:   1,
		Outcome:          OutcomeReproduced,
	}

	if result.GoDependencies != nil {
		t.Fatal("GoDependencies must be nil by default")
	}

	result.GoDependencies = &capsule.GoDependencyStatus{
		State:  capsule.GoDependenciesUnknown,
		Reason: "dependency availability not verified",
	}

	if result.Outcome != OutcomeReproduced {
		t.Fatal("dependency diagnostics changed replay outcome")
	}

	if err := result.GoDependencies.Validate(); err != nil {
		t.Fatalf("invalid dependency diagnostics: %v", err)
	}
}
