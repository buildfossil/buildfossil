package main

import (
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	splashReset   = "\x1b[0m"
	splashPurple  = "\x1b[38;2;168;85;247m"
	splashTeal    = "\x1b[38;2;20;184;166m"
	splashEmerald = "\x1b[38;2;16;185;129m"
)

var splashLetters = [5][3]string{
	{"██████╗ ", "███████╗", "██╗    ██╗"},
	{"██╔══██╗", "██╔════╝", "██║    ██║"},
	{"██████╔╝", "█████╗  ", "██║ █╗ ██║"},
	{"██╔══██╗", "██╔══╝  ", "██║███╗██║"},
	{"╚═╝  ╚═╝", "╚══════╝", " ╚══╝╚══╝"},
}

func splashSupportsColor(output *os.File) bool {
	if os.Getenv("NO_COLOR") != "" ||
		os.Getenv("CLICOLOR") == "0" ||
		os.Getenv("TERM") == "dumb" {
		return false
	}

	return splashIsTerminal(output)
}

func writeSplash(w io.Writer, version string, color bool) {
	for _, letters := range splashLetters {
		if color {
			fmt.Fprint(w, splashPurple, letters[0])
			fmt.Fprint(w, splashTeal, letters[1])
			fmt.Fprint(w, splashEmerald, letters[2])
			fmt.Fprint(w, splashReset)
		} else {
			fmt.Fprint(w, strings.Join(letters[:], ""))
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, "Capture failed Go builds. Replay them offline in Docker.")
	fmt.Fprintf(w, "Version: %s | Engine: Docker\n", version)
}

func PrintSplash(version string) {
	writeSplash(os.Stdout, version, splashSupportsColor(os.Stdout))
}
