package replay

import "bytes"

type FailureDifference string

const (
	DifferenceExitCode FailureDifference = "exit_code"
	DifferenceStderr   FailureDifference = "stderr"
)

func DiagnoseFailureDifferenceV3(
	outcome Outcome,
	originalExit int,
	originalStderr string,
	replayExit int,
	replayStderr []byte,
) FailureDifference {
	if outcome != OutcomeDifferent {
		return ""
	}

	if originalExit != replayExit {
		return DifferenceExitCode
	}

	if !bytes.Equal([]byte(originalStderr), replayStderr) {
		return DifferenceStderr
	}

	return ""
}
