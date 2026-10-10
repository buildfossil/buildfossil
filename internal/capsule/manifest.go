package capsule

import (
	"errors"
	"fmt"
)

const (
	SchemaVersionV1 = 1
	SchemaVersionV2 = 2

	// SchemaVersion remains v1 until the v2 writer and reader are ready.
	SchemaVersion = SchemaVersionV1
)

type Manifest struct {
	SchemaVersion int       `json:"schema_version"`
	Execution     Execution `json:"execution"`
	Platform      Platform  `json:"platform"`
	Workspace     Workspace `json:"workspace"`
	Runtime       *Runtime  `json:"runtime,omitempty"`
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

type Runtime struct {
	Kind    string `json:"kind"`
	Version string `json:"version"`
}

func (m Manifest) Validate() error {
	if m.SchemaVersion != SchemaVersionV1 &&
		m.SchemaVersion != SchemaVersionV2 {
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

	if m.Runtime != nil {
		if m.SchemaVersion != SchemaVersionV2 {
			return errors.New("capsule: runtime requires schema version 2")
		}

		if err := m.Runtime.Validate(); err != nil {
			return err
		}
	}

	var workspaceErr error

	switch m.SchemaVersion {
	case SchemaVersionV1:
		workspaceErr = m.Workspace.Validate()

	case SchemaVersionV2:
		workspaceErr = validateWorkspaceV2(m.Workspace)
	}

	if workspaceErr != nil {
		return fmt.Errorf("capsule: invalid workspace: %w", workspaceErr)
	}

	return nil
}
