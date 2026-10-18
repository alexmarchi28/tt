// Package resource finds the themes, word lists and quotes of tt.
package resource

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"sync"

	"github.com/alexmarchi28/tt"
)

// ErrNotFound is returned for resources which can't be found.
var ErrNotFound = errors.New("no such resource")

// Layers finds a resource by name: "-" is stdin, then a file of that path,
// then a file of that name in one of its layers, the first ones taking
// precedence.
type Layers struct {
	dirs     []fs.FS // Each holds a directory per type of resource.
	stdin    *onceReader
	readFile func(path string) ([]byte, error)
}

// New returns the resources of the user whose home directory is home: those
// in ~/.tt, then in /etc/tt, then the builtin ones.
func New(home string, stdin io.Reader) *Layers {
	return NewLayers(stdin, os.DirFS(filepath.Join(home, ".tt")), os.DirFS("/etc/tt"), tt.Builtin)
}

// NewLayers returns the resources of the given layers, each holding a
// directory per type of resource, the first ones taking precedence.
func NewLayers(stdin io.Reader, layers ...fs.FS) *Layers {
	return &Layers{dirs: layers, stdin: &onceReader{r: stdin}, readFile: os.ReadFile}
}

// onceReader reads r entirely the first time it is read and returns the same
// data afterwards. Stdin can only be read once, but the same resource may be
// needed more than once, e.g. the word list of adaptive tests.
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

// Read returns the resource of the given type (themes, words or quotes) and
// name.
func (r *Layers) Read(typ, name string) ([]byte, error) {
	if name == "-" {
		return r.stdin.readAll()
	}

	if b, err := r.readFile(name); err == nil {
		return b, nil
	}

	for _, dir := range r.dirs {
		if b, err := fs.ReadFile(dir, path.Join(typ, name)); err == nil {
			return b, nil
		}
	}

	return nil, ErrNotFound
}

// List returns the names of the resources of the given type, in alphabetical
// order.
func (r *Layers) List(typ string) []string {
	names := map[string]struct{}{}

	for _, dir := range r.dirs {
		entries, err := fs.ReadDir(dir, typ)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				names[entry.Name()] = struct{}{}
			}
		}
	}

	list := make([]string, 0, len(names))
	for name := range names {
		list = append(list, name)
	}

	sort.Strings(list)
	return list
}

// Index returns the position of the resource called name in names, also
// found by the base name of a path, or -1 if it is not there.
func Index(names []string, name string) int {
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
