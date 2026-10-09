package replay

import "testing"

func TestCompareFailure(t *testing.T) {
	tests := []struct {
		name              string
		originalExit      int
		originalStderr    string
		originalTruncated bool
		replayExit        int
		replayStderr      string
		replayTruncated   bool
		want              Outcome
		wantErr           bool
	}{
		{
			name:           "same failure",
			originalExit:   17,
			originalStderr: "BUILD_FOSSIL_TEST_FAILURE\n",
			replayExit:     17,
			replayStderr:   "BUILD_FOSSIL_TEST_FAILURE\n",
			want:           OutcomeReproduced,
		},
		{
			name:           "different error message",
			originalExit:   17,
			originalStderr: "BUILD_FOSSIL_TEST_FAILURE\n",
			replayExit:     17,
			replayStderr:   "DIFFERENT_FAILURE\n",
			want:           OutcomeDifferent,
		},
		{
			name:           "different exit code",
			originalExit:   17,
			originalStderr: "BUILD_FOSSIL_TEST_FAILURE\n",
			replayExit:     1,
			replayStderr:   "BUILD_FOSSIL_TEST_FAILURE\n",
			want:           OutcomeDifferent,
		},
		{
			name:           "replay passed",
			originalExit:   17,
			originalStderr: "BUILD_FOSSIL_TEST_FAILURE\n",
			replayExit:     0,
			want:           OutcomePassed,
		},
		{
			name:              "original output truncated",
			originalExit:      17,
			originalStderr:    "BUILD_FOSSIL_TEST_FAILURE\n",
			originalTruncated: true,
			replayExit:        17,
			replayStderr:      "BUILD_FOSSIL_TEST_FAILURE\n",
			want:              OutcomeInconclusive,
		},
		{
			name:            "replay output truncated",
			originalExit:    17,
			originalStderr:  "BUILD_FOSSIL_TEST_FAILURE\n",
			replayExit:      17,
			replayStderr:    "BUILD_FOSSIL_TEST_FAILURE\n",
			replayTruncated: true,
			want:            OutcomeInconclusive,
		},
		{
			name:         "missing original diagnostics",
			originalExit: 17,
			replayExit:   17,
			want:         OutcomeInconclusive,
		},
		{
			name:           "original command passed",
			originalExit:   0,
			originalStderr: "output\n",
			replayExit:     0,
			want:           OutcomeInconclusive,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CompareFailure(
				tt.originalExit,
				tt.originalStderr,
				tt.originalTruncated,
				tt.replayExit,
				[]byte(tt.replayStderr),
				tt.replayTruncated,
			)

			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr = %v", err, tt.wantErr)
			}

			if got != tt.want {
				t.Errorf("outcome = %q, want %q", got, tt.want)
			}
		})
	}
}
