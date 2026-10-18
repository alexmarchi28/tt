// Package internal holds the packages of tt other than its command.
package internal

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const module = "github.com/alexmarchi28/tt"

// allowedImports are the packages of the module, or third-party ones, each
// internal package may import besides the standard library. Their
// dependencies point towards the domain: typing imports nothing else, and
// none of them knows the screen or the command.
var allowedImports = map[string][]string{
	"typing":   nil,
	"history":  nil,
	"keystats": {module + "/internal/typing"},
	"resource": {module},
}

// imports returns the non-standard packages imported by the non-test files
// of dir.
func imports(t *testing.T, dir string) []string {
	t.Helper()

	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatal(err)
	}

	seen := map[string]bool{}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}

		file, err := parser.ParseFile(token.NewFileSet(), f, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range file.Imports {
			path, _ := strconv.Unquote(imp.Path.Value)
			if first := strings.Split(path, "/")[0]; strings.Contains(first, ".") {
				seen[path] = true
			}
		}
	}

	var paths []string
	for path := range seen {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	return paths
}

func TestPackageBoundaries(t *testing.T) {
	t.Parallel()

	dirs, err := filepath.Glob("*")
	if err != nil {
		t.Fatal(err)
	}

	for _, dir := range dirs {
		if matches, _ := filepath.Glob(filepath.Join(dir, "*.go")); len(matches) == 0 {
			continue
		}

		allowed, ok := allowedImports[dir]
		if !ok {
			t.Errorf("internal/%s has no import rules, add them to allowedImports", dir)
			continue
		}

		for _, path := range imports(t, dir) {
			if !contains(allowed, path) {
				t.Errorf("internal/%s imports %s, allowed: %q", dir, path, allowed)
			}
		}
	}
}

func contains(list []string, s string) bool {
	for _, candidate := range list {
		if candidate == s {
			return true
		}
	}

	return false
}
