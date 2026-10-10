package main

import (
	"embed"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
)

// builtin holds the resources shipped with tt. Explicit patterns are used so
// that files starting with an underscore (e.g. themes/_base) are included.
//
//go:embed themes/* words/* quotes/*
var builtin embed.FS

var configDirs []string

func init() {
	home, _ := os.LookupEnv("HOME")

	configDirs = []string{
		filepath.Join(home, ".tt"),
		"/etc/tt",
	}
}

func readResource(typ, name string) []byte {
	if name == "-" {
		b, err := io.ReadAll(os.Stdin)
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

	if b, err := builtin.ReadFile(path.Join(typ, name)); err == nil {
		return b
	}

	return nil
}

func listResourceNames(typ string) []string {
	names := map[string]struct{}{}

	if entries, err := builtin.ReadDir(typ); err == nil {
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
