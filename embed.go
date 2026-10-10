// Package tt holds the resources shipped with the tt typing test. The program
// itself lives in cmd/tt.
package tt

import "embed"

// Builtin holds the builtin themes, word lists and quotes. Explicit patterns
// are used so that files starting with an underscore (e.g. themes/_base) are
// included.
//
//go:embed themes/* words/* quotes/*
var Builtin embed.FS
