package main

import (
	"runtime/debug"
	"testing"
)

func TestResolveVersion(t *testing.T) {
	module := func(v string) func() (*debug.BuildInfo, bool) {
		return func() (*debug.BuildInfo, bool) {
			return &debug.BuildInfo{Main: debug.Module{Version: v}}, true
		}
	}
	none := func() (*debug.BuildInfo, bool) { return nil, false }

	tests := []struct {
		name      string
		built     string
		buildInfo func() (*debug.BuildInfo, bool)
		want      string
	}{
		{"set at build time", "v1.0.0", module("v0.9.0"), "1.0.0"},
		{"described by git", "v1.0.0-3-gabcdef1-dirty", none, "1.0.0-3-gabcdef1-dirty"},
		{"go install", "", module("v1.0.0"), "1.0.0"},
		{"local build", "", module("(devel)"), "dev"},
		{"no build information", "", none, "dev"},
		{"empty module version", "", module(""), "dev"},
	}

	for _, tc := range tests {
		if got := resolveVersion(tc.built, tc.buildInfo); got != tc.want {
			t.Errorf("%s: resolveVersion() = %q, want %q", tc.name, got, tc.want)
		}
	}
}
