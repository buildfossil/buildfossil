package capsule

import "fmt"

type GoDependencyState string

const (
	GoDependenciesReady       GoDependencyState = "ready"
	GoDependenciesUnavailable GoDependencyState = "unavailable"
	GoDependenciesUnknown     GoDependencyState = "unknown"
)

type GoDependencyStatus struct {
	State  GoDependencyState `json:"state"`
	Reason string            `json:"reason,omitempty"`
}

func (s GoDependencyStatus) Validate() error {
	switch s.State {
	case GoDependenciesReady, GoDependenciesUnknown:
		return nil

	case GoDependenciesUnavailable:
		if s.Reason == "" {
			return fmt.Errorf("capsule: unavailable Go dependencies require a reason")
		}
		return nil

	default:
		return fmt.Errorf("capsule: invalid Go dependency state %q", s.State)
	}
}
