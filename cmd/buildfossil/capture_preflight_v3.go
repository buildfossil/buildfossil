package main

import (
	"fmt"
	"os"
)

func checkCaptureOutputAvailableV3(output string) error {
	if output == "" {
		return fmt.Errorf("empty output path")
	}

	_, err := os.Lstat(output)

	if err == nil {
		return fmt.Errorf(
			"output already exists: %s; move or rename the existing capsule before retrying",
			output,
		)
	}

	if !os.IsNotExist(err) {
		return fmt.Errorf("check output %q: %w", output, err)
	}

	return nil
}
