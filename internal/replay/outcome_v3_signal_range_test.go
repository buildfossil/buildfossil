package replay

import "testing"

func TestCompareFailureV3AmbiguousSignalExitRange(t *testing.T) {
	tests := []struct {
		name string
		exit int
		want Outcome
	}{
		{"ordinary_failure_1", 1, OutcomeReproduced},
		{"ordinary_failure_2", 2, OutcomeReproduced},
		{"outside_range_128", 128, OutcomeReproduced},
		{"signal_lower_bound_129", 129, OutcomeInconclusive},
		{"sigkill_137", 137, OutcomeInconclusive},
		{"sigsegv_139", 139, OutcomeInconclusive},
		{"sigterm_143", 143, OutcomeInconclusive},
		{"signal_upper_bound_159", 159, OutcomeInconclusive},
		{"outside_range_160", 160, OutcomeReproduced},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := compareFailureV3(
				tt.exit,
				"build failed\n",
				false,
				tt.exit,
				[]byte("build failed\n"),
				false,
			)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("exit=%d outcome=%q, want=%q",
					tt.exit, got, tt.want)
			}
		})
	}
}

func TestCompareFailureV3SignalRangePreservesDifferences(t *testing.T) {
	tests := []struct {
		name           string
		originalExit   int
		replayExit     int
		originalStderr string
		replayStderr   string
		want           Outcome
	}{
		{
			name:           "different_exit",
			originalExit:   143,
			replayExit:     139,
			originalStderr: "failure\n",
			replayStderr:   "failure\n",
			want:           OutcomeDifferent,
		},
		{
			name:           "different_stderr",
			originalExit:   143,
			replayExit:     143,
			originalStderr: "original\n",
			replayStderr:   "different\n",
			want:           OutcomeDifferent,
		},
		{
			name:           "replay_passed",
			originalExit:   143,
			replayExit:     0,
			originalStderr: "failure\n",
			want:           OutcomePassed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := compareFailureV3(
				tt.originalExit,
				tt.originalStderr,
				false,
				tt.replayExit,
				[]byte(tt.replayStderr),
				false,
			)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("outcome=%q, want=%q", got, tt.want)
			}
		})
	}
}

func TestCompareFailureSharedOracleSignalExitUnchanged(t *testing.T) {
	got, err := CompareFailure(
		143,
		"build failed\n",
		false,
		143,
		[]byte("build failed\n"),
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	if got != OutcomeReproduced {
		t.Fatalf("shared oracle outcome=%q, want reproduced", got)
	}
}
