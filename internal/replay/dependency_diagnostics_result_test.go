package replay

import (
	"testing"

	"github.com/buildfossil/buildfossil/internal/capsule"
)

func TestAttachGoDependencyDiagnosticsPreservesOutcome(t *testing.T) {
	original := Result{
		OriginalExitCode: 1,
		ReplayExitCode:   1,
		Outcome:          OutcomeReproduced,
	}

	status := capsule.GoDependencyStatus{
		State:  capsule.GoDependenciesUnknown,
		Reason: "Docker dependency probe failed",
	}

	got := attachGoDependencyDiagnostics(original, status)

	if got.Outcome != original.Outcome {
		t.Fatalf("outcome changed: %q -> %q", original.Outcome, got.Outcome)
	}

	if got.OriginalExitCode != original.OriginalExitCode ||
		got.ReplayExitCode != original.ReplayExitCode {
		t.Fatal("diagnostics changed exit codes")
	}

	if got.GoDependencies == nil ||
		got.GoDependencies.State != capsule.GoDependenciesUnknown {
		t.Fatal("missing unknown dependency diagnostics")
	}
}
