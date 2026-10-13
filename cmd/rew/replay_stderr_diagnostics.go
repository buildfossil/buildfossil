package main

import (
	"fmt"
	"io"
)

const maxDisplayedStderr = 16 << 10

func printReplayStderrDiagnostics(
	w io.Writer,
	original string,
	originalTruncated bool,
	replayed []byte,
	replayedTruncated bool,
	originalDisplayTruncated bool,
	replayDisplayTruncated bool,
) {
	fmt.Fprintln(w, "\nWARNING: stderr may contain sensitive information.")

	printStderrSection(w, "Original stderr", []byte(original), originalTruncated, originalDisplayTruncated)
	printStderrSection(w, "Replay stderr", replayed, replayedTruncated, replayDisplayTruncated)
}

func printStderrSection(
	w io.Writer,
	title string,
	data []byte,
	captureTruncated bool,
	preTruncated bool,
) {
	fmt.Fprintf(w, "\n=== %s ===\n", title)

	displayed := data
	displayTruncated := preTruncated || len(data) > maxDisplayedStderr

	if len(data) > maxDisplayedStderr {
		displayed = data[:maxDisplayedStderr]
	}

	if len(displayed) == 0 {
		fmt.Fprintln(w, "(empty)")
	} else {
		_, _ = w.Write(displayed)
		if displayed[len(displayed)-1] != '\n' {
			fmt.Fprintln(w)
		}
	}

	if displayTruncated {
		fmt.Fprintln(w, "[display truncated to 16 KiB]")
	}

	if captureTruncated {
		fmt.Fprintln(w, "[stderr truncated during capture or replay]")
	}
}
