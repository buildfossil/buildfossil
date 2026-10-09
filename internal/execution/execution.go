package execution

import (
	"errors"
	"io"
	"os"
	"os/exec"
)

type Result struct {
	Argv            []string `json:"argv"`
	ExitCode        int      `json:"exit_code"`
	Stdout          string   `json:"stdout"`
	Stderr          string   `json:"stderr"`
	StdoutTruncated bool     `json:"stdout_truncated"`
	StderrTruncated bool     `json:"stderr_truncated"`
}

func Run(argv []string, stdout, stderr io.Writer) (Result, error) {
	return RunInDir(argv, "", stdout, stderr)
}

func RunInDir(
	argv []string,
	dir string,
	stdout, stderr io.Writer,
) (Result, error) {
	if len(argv) == 0 {
		return Result{}, errors.New("execution: empty command")
	}

	stdoutBuffer := newLimitedWriter(MaxCapturedOutput)
	stderrBuffer := newLimitedWriter(MaxCapturedOutput)

	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Stdout = io.MultiWriter(stdout, stdoutBuffer)
	cmd.Stderr = io.MultiWriter(stderr, stderrBuffer)
	cmd.Stdin = os.Stdin

	err := cmd.Run()

	result := Result{
		Argv:            append([]string(nil), argv...),
		Stdout:          stdoutBuffer.String(),
		Stderr:          stderrBuffer.String(),
		StdoutTruncated: stdoutBuffer.Truncated(),
		StderrTruncated: stderrBuffer.Truncated(),
	}

	if err == nil {
		return result, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result, nil
	}

	return result, err
}
