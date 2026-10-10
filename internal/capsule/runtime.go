package capsule

import "fmt"

const SupportedGoVersion = "1.26.6"

func (r Runtime) Validate() error {
	if r.Kind != "go" {
		return fmt.Errorf("capsule: unsupported runtime kind: %q", r.Kind)
	}

	if r.Version != SupportedGoVersion {
		return fmt.Errorf(
			"capsule: unsupported Go runtime version: %q",
			r.Version,
		)
	}

	return nil
}
