package capsule

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const MaxGoDependencyDiagnosticsSize int64 = 8 << 20

type goListPackage struct {
	Incomplete bool `json:"Incomplete"`

	Error *struct {
		Err string `json:"Err"`
	} `json:"Error"`

	DepsErrors []struct {
		Err string `json:"Err"`
	} `json:"DepsErrors"`
}

// InspectGoListOutput analyzes the structured output of
// "go list -deps -e -json ./...".
//
// It does not execute Go, access the network, or change files.
func InspectGoListOutput(r io.Reader) (GoDependencyStatus, error) {
	limited := &io.LimitedReader{
		R: r,
		N: MaxGoDependencyDiagnosticsSize + 1,
	}

	decoder := json.NewDecoder(limited)

	foundPackage := false
	foundError := false

	for {
		var pkg goListPackage

		err := decoder.Decode(&pkg)

		if err == io.EOF {
			break
		}

		if err != nil {
			return GoDependencyStatus{}, fmt.Errorf(
				"decode go list output: %w",
				err,
			)
		}

		foundPackage = true

		if pkg.Error != nil && strings.TrimSpace(pkg.Error.Err) != "" {
			foundError = true
		}

		for _, depErr := range pkg.DepsErrors {
			if strings.TrimSpace(depErr.Err) != "" {
				foundError = true
			}
		}

		if pkg.Incomplete {
			foundError = true
		}
	}

	if limited.N == 0 {
		return GoDependencyStatus{}, fmt.Errorf(
			"go list output exceeds %d bytes",
			MaxGoDependencyDiagnosticsSize,
		)
	}

	if !foundPackage {
		return GoDependencyStatus{
			State:  GoDependenciesUnknown,
			Reason: "go list returned no packages",
		}, nil
	}

	if foundError {
		return GoDependencyStatus{
			State:  GoDependenciesUnknown,
			Reason: "go list reported incomplete packages or dependency errors",
		}, nil
	}

	return GoDependencyStatus{
		State: GoDependenciesReady,
	}, nil
}
