package replay

// compareFailureV3 conservatively treats matching exit codes 129-159
// as inconclusive. These codes may represent signal termination under
// the common 128+signal convention, but Docker ExecInspect does not
// distinguish signals from explicit process exit codes.
// This is a heuristic, not a signal or OOM detector.
func compareFailureV3(
	originalExit int,
	originalStderr string,
	originalTruncated bool,
	replayExit int,
	replayStderr []byte,
	replayTruncated bool,
) (Outcome, error) {
	outcome, err := CompareFailure(
		originalExit,
		originalStderr,
		originalTruncated,
		replayExit,
		replayStderr,
		replayTruncated,
	)
	if err != nil {
		return outcome, err
	}

	if outcome == OutcomeReproduced && replayExit >= 129 && replayExit <= 159 {
		return OutcomeInconclusive, nil
	}

	return outcome, nil
}
