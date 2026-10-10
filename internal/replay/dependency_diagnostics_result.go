package replay

import "github.com/buildfossil/buildfossil/internal/capsule"

// attachGoDependencyDiagnostics attaches advisory information without
// changing the outcome or exit codes of a completed replay.
func attachGoDependencyDiagnostics(
	result Result,
	status capsule.GoDependencyStatus,
) Result {
	result.GoDependencies = &status
	return result
}
