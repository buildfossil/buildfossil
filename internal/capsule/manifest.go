package capsule

import (
	"errors"
	"fmt"
)

const (
	SchemaVersionV1 = 1
	SchemaVersionV2 = 2
	SchemaVersionV3 = 3

	// SchemaVersion remains v1 until the v2 writer and reader are ready.
	SchemaVersion = SchemaVersionV1
)

type Manifest struct {
	SchemaVersion  int                 `json:"schema_version"`
	Execution      Execution           `json:"execution"`
	Platform       Platform            `json:"platform"`
	Workspace      Workspace           `json:"workspace"`
	Runtime        *Runtime            `json:"runtime,omitempty"`
	GoBuildEnv     *GoBuildEnvironment `json:"go_build_env,omitempty"`
	GoDependencies *GoDependencyStatus `json:"go_dependencies,omitempty"`
	GoModules      []GoModuleV3        `json:"go_modules,omitempty"`
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
		m.SchemaVersion != SchemaVersionV2 &&
		m.SchemaVersion != SchemaVersionV3 {
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
		if m.SchemaVersion < SchemaVersionV2 {
			return errors.New("capsule: runtime requires schema version 2")
		}

		if err := m.Runtime.Validate(); err != nil {
			return err
		}
	}

	if m.GoBuildEnv != nil {
		if m.SchemaVersion < SchemaVersionV2 {
			return errors.New("capsule: Go build environment requires schema version 2")
		}

		if m.Runtime == nil ||
			m.Runtime.Kind != "go" ||
			m.Runtime.Version != SupportedGoVersion {
			return errors.New("capsule: Go build environment requires supported Go runtime")
		}

		if m.GoBuildEnv.GOOS == "" ||
			m.GoBuildEnv.GOARCH == "" ||
			(m.GoBuildEnv.CGOEnabled != "0" && m.GoBuildEnv.CGOEnabled != "1") {
			return errors.New("capsule: invalid Go build environment")
		}
	}

	if m.GoDependencies != nil {
		if m.SchemaVersion < SchemaVersionV2 {
			return errors.New(
				"capsule: Go dependency status requires schema version 2",
			)
		}

		if m.Runtime == nil ||
			m.Runtime.Kind != "go" ||
			m.Runtime.Version != SupportedGoVersion {
			return errors.New(
				"capsule: Go dependency status requires supported Go runtime",
			)
		}

		if err := m.GoDependencies.Validate(); err != nil {
			return err
		}
	}

	if m.SchemaVersion != SchemaVersionV3 && len(m.GoModules) != 0 {
		return errors.New("capsule: Go modules require schema version 3")
	}

	if m.SchemaVersion == SchemaVersionV3 {
		if m.Runtime == nil ||
			m.Runtime.Kind != "go" ||
			m.Runtime.Version != SupportedGoVersion {
			return errors.New("capsule: schema version 3 requires supported Go runtime")
		}

		if m.GoBuildEnv == nil {
			return errors.New("capsule: schema version 3 requires Go build environment")
		}

		if err := ValidateGoModulesV3(m.GoModules); err != nil {
			return err
		}
	}

	var workspaceErr error

	switch m.SchemaVersion {
	case SchemaVersionV1:
		workspaceErr = m.Workspace.Validate()

	case SchemaVersionV2, SchemaVersionV3:
		workspaceErr = validateWorkspaceV2(m.Workspace)
	}

	if workspaceErr != nil {
		return fmt.Errorf("capsule: invalid workspace: %w", workspaceErr)
	}

	return nil
}
