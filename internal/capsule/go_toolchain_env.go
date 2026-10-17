package capsule

import (
	"os"
	"strings"
)

// LocalGoToolchainEnv preserves the process environment while ensuring
// exactly one GOTOOLCHAIN=local entry.
func LocalGoToolchainEnv() []string {
	environ := os.Environ()
	result := make([]string, 0, len(environ)+1)

	for _, entry := range environ {
		key, _, _ := strings.Cut(entry, "=")
		if key == "GOTOOLCHAIN" {
			continue
		}
		result = append(result, entry)
	}

	return append(result, "GOTOOLCHAIN=local")
}
