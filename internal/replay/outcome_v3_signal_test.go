package replay

import "testing"

func TestCompareFailureV3AmbiguousExit137(t *testing.T) {
	tests := []struct {
		name         string
		originalExit int
		replayExit   int
		originalErr  string
		replayErr    string
		want         Outcome
	}{
		{
			name:         "matching exit 137 is inconclusive",
			originalExit: 137,
			replayExit:   137,
			originalErr:  "killed\n",
			replayErr:    "killed\n",
			want:         OutcomeInconclusive,
		},
		{
			name:         "normal matching failure reproduces",
			originalExit: 1,
			replayExit:   1,
			originalErr:  "compile error\n",
			replayErr:    "compile error\n",
			want:         OutcomeReproduced,
		},
		{
			name:         "different failure stays different",
			originalExit: 1,
			replayExit:   2,
			originalErr:  "compile error\n",
			replayErr:    "compile error\n",
			want:         OutcomeDifferent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := compareFailureV3(
				tt.originalExit,
				tt.originalErr,
				false,
				tt.replayExit,
				[]byte(tt.replayErr),
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

func TestCompareFailureV3PreservesOracleSemantics(t *testing.T) {
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
			name:           "different stderr with exit 137",
			originalExit:   137,
			originalStderr: "original\n",
			replayExit:     137,
			replayStderr:   "different\n",
			want:           OutcomeDifferent,
		},
		{
			name:           "different exit codes",
			originalExit:   1,
			originalStderr: "failure\n",
			replayExit:     137,
			replayStderr:   "failure\n",
			want:           OutcomeDifferent,
		},
		{
			name:           "invalid original exit",
			originalExit:   0,
			originalStderr: "failure\n",
			replayExit:     137,
			replayStderr:   "failure\n",
			want:           OutcomeInconclusive,
			wantErr:        true,
		},
		{
			name:              "truncated original stderr",
			originalExit:      137,
			originalStderr:    "failure\n",
			originalTruncated: true,
			replayExit:        137,
			replayStderr:      "failure\n",
			want:              OutcomeInconclusive,
		},
		{
			name:            "truncated replay stderr",
			originalExit:    137,
			originalStderr:  "failure\n",
			replayExit:      137,
			replayStderr:    "failure\n",
			replayTruncated: true,
			want:            OutcomeInconclusive,
		},
		{
			name:           "matching exit 137 remains inconclusive in v3",
			originalExit:   137,
			originalStderr: "failure\n",
			replayExit:     137,
			replayStderr:   "failure\n",
			want:           OutcomeInconclusive,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outcome, err := compareFailureV3(
				tt.originalExit,
				tt.originalStderr,
				tt.originalTruncated,
				tt.replayExit,
				[]byte(tt.replayStderr),
				tt.replayTruncated,
			)

			if (err != nil) != tt.wantErr {
				t.Fatalf("err=%v, wantErr=%v", err, tt.wantErr)
			}
			if outcome != tt.want {
				t.Fatalf("outcome=%q, want=%q", outcome, tt.want)
			}
		})
	}
}

// Replay v2 still uses the shared oracle. A matching exit code 137
// must retain its original classification outside the v3 policy.
func TestCompareFailureSharedOracleExit137Unchanged(t *testing.T) {
	got, err := CompareFailure(
		137,
		"failure\n",
		false,
		137,
		[]byte("failure\n"),
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	if got != OutcomeReproduced {
		t.Fatalf("outcome=%q, want=%q", got, OutcomeReproduced)
	}
}
