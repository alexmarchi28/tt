package main

import (
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"sync"

	"github.com/alexmarchi28/tt"
)

var configDirs []string

func init() {
	home, _ := os.UserHomeDir()

	configDirs = []string{
		filepath.Join(home, ".tt"),
		"/etc/tt",
	}
}

// onceReader reads r entirely the first time it is read and returns the same
// data afterwards.
type onceReader struct {
	r    io.Reader
	once sync.Once
	data []byte
	err  error
}

func (o *onceReader) readAll() ([]byte, error) {
	o.once.Do(func() { o.data, o.err = io.ReadAll(o.r) })
	return o.data, o.err
}

// stdinResource is the resource named "-". Stdin can only be read once, but
// the same resource may be needed more than once, e.g. the word list of
// adaptive tests.
var stdinResource = &onceReader{r: os.Stdin}

func readResource(typ, name string) []byte {
	if name == "-" {
		b, err := stdinResource.readAll()
		if err != nil {
			panic(err)
		}

		return b
	}

	if b, err := os.ReadFile(name); err == nil {
		return b
	}

	for _, d := range configDirs {
		if b, err := os.ReadFile(filepath.Join(d, typ, name)); err == nil {
			return b
		}
	}

	if b, err := tt.Builtin.ReadFile(path.Join(typ, name)); err == nil {
		return b
	}

	return nil
}

func listResourceNames(typ string) []string {
	names := map[string]struct{}{}

	if entries, err := tt.Builtin.ReadDir(typ); err == nil {
		for _, entry := range entries {
			names[entry.Name()] = struct{}{}
		}
	}

	for _, d := range configDirs {
		entries, err := os.ReadDir(filepath.Join(d, typ))
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			names[entry.Name()] = struct{}{}
		}
	}

	list := make([]string, 0, len(names))
	for name := range names {
		list = append(list, name)
	}

	sort.Strings(list)
	return list
}

func indexResourceName(names []string, name string) int {
	for i, candidate := range names {
		if candidate == name {
			return i
		}
	}

	base := filepath.Base(name)
	if base != name {
		for i, candidate := range names {
			if candidate == base {
				return i
			}
		}
	}

	return -1
}
