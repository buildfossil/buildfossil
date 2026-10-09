package capsule

import (
	"errors"
	"fmt"
)

const SchemaVersion = 1

type Manifest struct {
	SchemaVersion int       `json:"schema_version"`
	Execution     Execution `json:"execution"`
	Platform      Platform  `json:"platform"`
	Workspace     Workspace `json:"workspace"`
}

type Workspace struct {
	Files []FileMetadata `json:"files"`
}

type FileMetadata struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type Execution struct {
	Argv            []string `json:"argv"`
	WorkingDir      string   `json:"working_dir"`
	ExitCode        int      `json:"exit_code"`
	Stdout          string   `json:"stdout"`
	Stderr          string   `json:"stderr"`
	StdoutTruncated bool     `json:"stdout_truncated"`
	StderrTruncated bool     `json:"stderr_truncated"`
}

type Platform struct {
	OS           string `json:"os"`
	Architecture string `json:"architecture"`
}

func (m Manifest) Validate() error {
	if m.SchemaVersion != SchemaVersion {
		return errors.New("capsule: unsupported schema version")
	}

	if len(m.Execution.Argv) == 0 {
		return errors.New("capsule: missing execution command")
	}

	if m.Execution.Argv[0] == "" {
		return errors.New("capsule: empty executable")
	}

	if m.Platform.OS == "" || m.Platform.Architecture == "" {
		return errors.New("capsule: missing platform")
	}

	if err := m.Workspace.Validate(); err != nil {
		return fmt.Errorf("capsule: invalid workspace: %w", err)
	}

	return nil
}
