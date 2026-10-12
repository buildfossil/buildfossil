package replay

// compareFailureV3 conservatively treats replay exit code 137 as
// inconclusive. Docker can return 137 when an exec process is killed,
// including by OOM, without identifying the cause in ExecInspect.
// This is not an OOM detector.
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

	if outcome == OutcomeReproduced && replayExit == 137 {
		return OutcomeInconclusive, nil
	}

	return outcome, nil
}
