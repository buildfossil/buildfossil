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

	if runtime.Kind == "go" && runtime.Version == "1.26.6" {
		return goReplayImage, nil
	}

	return "", fmt.Errorf(
		"docker SDK: unsupported runtime %q version %q",
		runtime.Kind,
		runtime.Version,
	)
}
