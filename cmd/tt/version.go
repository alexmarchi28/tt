package main

import (
	"runtime/debug"
	"strings"
)

// version is set at build time with -ldflags "-X main.version=...", see the
// Makefile.
var version string

// versionString returns the version of tt, without its leading v.
func versionString() string {
	return resolveVersion(version, debug.ReadBuildInfo)
}

// resolveVersion returns the version set at build time, or else the version of
// the module recorded by go install, or else "dev".
func resolveVersion(built string, buildInfo func() (*debug.BuildInfo, bool)) string {
	v := built
	if v == "" {
		if info, ok := buildInfo(); ok && info.Main.Version != "(devel)" {
			v = info.Main.Version
		}
	}

	if v == "" {
		return "dev"
	}

	return strings.TrimPrefix(v, "v")
}
