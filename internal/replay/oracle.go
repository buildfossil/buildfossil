package replay

import (
	"bytes"
	"errors"
)

type Outcome string

const (
	OutcomeReproduced   Outcome = "reproduced"
	OutcomeDifferent    Outcome = "different_failure"
	OutcomePassed       Outcome = "passed"
	OutcomeInconclusive Outcome = "inconclusive"
)

func CompareFailure(
	originalExit int,
	originalStderr string,
	originalTruncated bool,
	replayExit int,
	replayStderr []byte,
	replayTruncated bool,
) (Outcome, error) {
	if originalTruncated || replayTruncated {
		return OutcomeInconclusive, nil
	}

	if originalExit == 0 {
		return OutcomeInconclusive,
			errors.New("oracle: original command did not fail")
	}

	if originalStderr == "" {
		return OutcomeInconclusive, nil
	}

	if replayExit == 0 {
		return OutcomePassed, nil
	}

	if originalExit != replayExit {
		return OutcomeDifferent, nil
	}

	if !bytes.Equal([]byte(originalStderr), replayStderr) {
		return OutcomeDifferent, nil
	}

	return OutcomeReproduced, nil
}
