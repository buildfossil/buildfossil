package replay

import "testing"

func TestDiagnoseFailureDifferenceV3(t *testing.T) {
	tests := []struct {
		name           string
		outcome        Outcome
		originalExit   int
		originalStderr string
		replayExit     int
		replayStderr   string
		want           FailureDifference
	}{
		{
			name:           "different stderr",
			outcome:        OutcomeDifferent,
			originalExit:   1,
			originalStderr: "original\n",
			replayExit:     1,
			replayStderr:   "different\n",
			want:           DifferenceStderr,
		},
		{
			name:           "different exit code",
			outcome:        OutcomeDifferent,
			originalExit:   1,
			originalStderr: "same\n",
			replayExit:     137,
			replayStderr:   "same\n",
			want:           DifferenceExitCode,
		},
		{
			name:           "exit code takes priority",
			outcome:        OutcomeDifferent,
			originalExit:   1,
			originalStderr: "original\n",
			replayExit:     137,
			replayStderr:   "different\n",
			want:           DifferenceExitCode,
		},
		{
			name:           "reproduced",
			outcome:        OutcomeReproduced,
			originalExit:   1,
			originalStderr: "same\n",
			replayExit:     1,
			replayStderr:   "same\n",
		},
		{
			name:           "passed",
			outcome:        OutcomePassed,
			originalExit:   1,
			originalStderr: "failed\n",
			replayExit:     0,
		},
		{
			name:           "inconclusive",
			outcome:        OutcomeInconclusive,
			originalExit:   1,
			originalStderr: "original\n",
			replayExit:     1,
			replayStderr:   "partial",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DiagnoseFailureDifferenceV3(
				tt.outcome,
				tt.originalExit,
				tt.originalStderr,
				tt.replayExit,
				[]byte(tt.replayStderr),
			)

			if got != tt.want {
				t.Fatalf("difference=%q, want=%q", got, tt.want)
			}
		})
	}
}
