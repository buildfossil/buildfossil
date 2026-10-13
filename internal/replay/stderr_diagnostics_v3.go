package replay

const maxStderrDiagnosticsV3 = 16 << 10

// StderrDiagnosticsV3 contains bounded, opt-in replay diagnostics.
// It does not affect the strict failure comparison.
type StderrDiagnosticsV3 struct {
	Original          string
	OriginalTruncated bool
	Replayed          []byte
	ReplayTruncated   bool

	OriginalDisplayTruncated bool
	ReplayDisplayTruncated   bool
}

func newStderrDiagnosticsV3(
	original string,
	originalTruncated bool,
	replayed []byte,
	replayedTruncated bool,
) *StderrDiagnosticsV3 {
	d := &StderrDiagnosticsV3{
		OriginalTruncated: originalTruncated,
		ReplayTruncated:   replayedTruncated,
	}

	if len(original) > maxStderrDiagnosticsV3 {
		d.OriginalDisplayTruncated = true
		original = original[:maxStderrDiagnosticsV3]
	}
	d.Original = original

	if len(replayed) > maxStderrDiagnosticsV3 {
		d.ReplayDisplayTruncated = true
		replayed = replayed[:maxStderrDiagnosticsV3]
	}
	d.Replayed = append([]byte(nil), replayed...)

	return d
}
