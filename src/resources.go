package main

import (
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

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

	return readPackedFile(filepath.Join(typ, name))
}

func listResourceNames(typ string) []string {
	names := map[string]struct{}{}
	prefix := typ + "/"

	for path := range packedFiles {
		if strings.Index(path, prefix) == 0 {
			_, name := filepath.Split(path)
			names[name] = struct{}{}
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
