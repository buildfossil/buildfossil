package capsule

import "fmt"

const (
	SupportedGoVersion    = "1.26.6"
	SupportedGoVersion125 = "1.25.14"
)

// IsSupportedGoVersion accepts only exact Go toolchain patch versions
// for which REW maintains a compatible replay runtime.
func IsSupportedGoVersion(version string) bool {
	switch version {
	case SupportedGoVersion, SupportedGoVersion125:
		return true
	default:
		return false
	}
}

func (r Runtime) Validate() error {
	if r.Kind != "go" {
		return fmt.Errorf("capsule: unsupported runtime kind: %q", r.Kind)
	}

	if !IsSupportedGoVersion(r.Version) {
		return fmt.Errorf(
			"capsule: unsupported Go runtime version: %q",
			r.Version,
		)
	}

	return nil
}
