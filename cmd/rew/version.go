package main

import (
	"runtime/debug"
	"strings"
)

// releaseVersion is set by official release builds using -ldflags.
var releaseVersion string

func selectRewVersion(injected, moduleVersion string) string {
	if v := strings.TrimSpace(injected); v != "" {
		return v
	}

	if moduleVersion == "" ||
		moduleVersion == "(devel)" ||
		strings.Contains(moduleVersion, "+dirty") ||
		!strings.HasPrefix(moduleVersion, "v") {
		return "dev"
	}

	return moduleVersion
}

func rewVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return selectRewVersion(releaseVersion, "")
	}

	return selectRewVersion(releaseVersion, info.Main.Version)
}
