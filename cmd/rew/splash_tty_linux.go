//go:build linux

package main

import (
	"os"

	"golang.org/x/sys/unix"
)

func splashIsTerminal(output *os.File) bool {
	if output == nil {
		return false
	}

	_, err := unix.IoctlGetTermios(int(output.Fd()), unix.TCGETS)
	return err == nil
}
