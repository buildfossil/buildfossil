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

	return runInDirWithExecutable(argv, argv[0], dir, stdout, stderr)
}

// RunInDirWithExecutable runs an explicitly selected executable while
// preserving the original argv in the execution record.
func RunInDirWithExecutable(
	argv []string,
	executable string,
	dir string,
	stdout, stderr io.Writer,
) (Result, error) {
	if len(argv) == 0 {
		return Result{}, errors.New("execution: empty command")
	}
	if executable == "" {
		return Result{}, errors.New("execution: empty executable")
	}

	return runInDirWithExecutable(argv, executable, dir, stdout, stderr)
}

// RunInDirWithExecutableEnv runs the selected executable with an
// explicitly supplied environment, preserving the original argv.
func RunInDirWithExecutableEnv(
	argv []string,
	executable string,
	dir string,
	env []string,
	stdout, stderr io.Writer,
) (Result, error) {
	if len(argv) == 0 {
		return Result{}, errors.New("execution: empty command")
	}
	if executable == "" {
		return Result{}, errors.New("execution: empty executable")
	}
	if env == nil {
		return Result{}, errors.New("execution: nil environment")
	}

	return runInDirWithExecutableEnv(
		argv, executable, dir, env, stdout, stderr,
	)
}

func runInDirWithExecutable(
	argv []string,
	executable string,
	dir string,
	stdout, stderr io.Writer,
) (Result, error) {
	return runInDirWithExecutableEnv(
		argv, executable, dir, nil, stdout, stderr,
	)
}

func runInDirWithExecutableEnv(
	argv []string,
	executable string,
	dir string,
	env []string,
	stdout, stderr io.Writer,
) (Result, error) {
	stdoutBuffer := newLimitedWriter(MaxCapturedOutput)
	stderrBuffer := newLimitedWriter(MaxCapturedOutput)

	cmd := exec.Command(executable, argv[1:]...)
	if env != nil {
		cmd.Env = append([]string(nil), env...)
	}
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
