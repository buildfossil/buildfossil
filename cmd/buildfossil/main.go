package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: buildfossil <command>")
		return 2
	}

	switch args[0] {
	case "--help", "-h", "help":
		if len(args) != 1 {
			fmt.Fprintln(os.Stderr, "Usage: buildfossil help")
			return 2
		}
		printGeneralHelp()
		return 0

	case "--version":
		if len(args) != 1 {
			fmt.Fprintln(os.Stderr, "Usage: buildfossil --version")
			return 2
		}
		fmt.Println("buildfossil " + buildfossilVersion())
		return 0
	case "capture-demo":
		return runCaptureDemo(args[1:])

	case "version":
		if len(args) != 1 {
			fmt.Fprintln(os.Stderr, "Usage: buildfossil version")
			return 2
		}
		fmt.Println("buildfossil " + buildfossilVersion())
		return 0

	case "capture":
		if len(args) == 2 && (args[1] == "--help" || args[1] == "-h") {
			printCaptureHelp()
			return 0
		}

		if len(args) >= 2 && args[1] == "--v3" {
			var includes []string
			includeGoEmbed := false
			i := 2

			for i < len(args) {
				switch args[i] {
				case "--include":
					if i+1 >= len(args) ||
						args[i+1] == "" ||
						strings.HasPrefix(args[i+1], "--") {
						fmt.Fprintln(os.Stderr,
							"buildfossil: --include requires a file path")
						return 2
					}
					includes = append(includes, args[i+1])
					i += 2

				case "--include-go-embed":
					if includeGoEmbed {
						fmt.Fprintln(os.Stderr,
							"buildfossil: duplicate --include-go-embed")
						return 2
					}
					includeGoEmbed = true
					i++

				default:
					goto captureV3OptionsDone
				}
			}
		captureV3OptionsDone:

			if len(includes) == 0 ||
				i >= len(args) ||
				args[i] != "--" ||
				i+1 >= len(args) {
				fmt.Fprintln(os.Stderr,
					"Usage: buildfossil capture --v3 "+
						"--include <file> ... -- <command> [args...]")
				return 2
			}

			return captureCommandV3WithEmbed(
				args[i+1:],
				".",
				includes,
				"failure.bfc",
				includeGoEmbed,
			)
		}

		if len(args) >= 3 && args[1] == "--" {
			return captureCommand(args[2:], "failure.bfc", false)
		}

		var includes []string
		i := 1

		for i < len(args) && args[i] == "--include" {
			if i+1 >= len(args) || args[i+1] == "" || args[i+1] == "--" {
				fmt.Fprintln(os.Stderr, "buildfossil: --include requires a file path")
				return 2
			}

			includes = append(includes, args[i+1])
			i += 2
		}

		if len(includes) == 0 || i >= len(args) || args[i] != "--" || i+1 >= len(args) {
			fmt.Fprintln(
				os.Stderr,
				"Usage: buildfossil capture [--include <file> ...] -- <command> [args...]",
			)
			return 2
		}

		return captureCommandV2(
			args[i+1:],
			".",
			includes,
			"failure.bfc",
		)

	case "replay":
		if len(args) == 2 && (args[1] == "--help" || args[1] == "-h") {
			printReplayHelp()
			return 0
		}
		return runReplay(args[1:])

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", args[0])
		return 2
	}
}
