package docker

import "fmt"

type RuntimeSpec struct {
	Kind    string
	Version string
}

func selectReplayImage(runtime *RuntimeSpec) (string, error) {
	if runtime == nil {
		return replayImage, nil
	}

	if runtime.Kind == "go" {
		switch runtime.Version {
		case "1.26.6":
			return goReplayImage, nil
		case "1.25.14":
			return goReplayImage125, nil
		}
	}

	return "", fmt.Errorf(
		"docker SDK: unsupported runtime %q version %q",
		runtime.Kind,
		runtime.Version,
	)
}

// isSupportedGoRuntime accepts only Go runtimes with a pinned
// replay image in the Docker runtime allowlist.
func isSupportedGoRuntime(runtime *RuntimeSpec) bool {
	if runtime == nil || runtime.Kind != "go" {
		return false
	}
	_, err := selectReplayImage(runtime)
	return err == nil
}
