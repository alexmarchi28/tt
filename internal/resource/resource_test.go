package resource

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/alexmarchi28/tt"
)

// newTestLayers returns the resources of the calling test: user resources in
// a directory of its own, whose path is returned (see writeUserResource),
// then the builtin ones. Resources named "-" are read from an empty stdin.
func newTestLayers(t *testing.T) (*Layers, string) {
	t.Helper()

	dir := t.TempDir()
	return NewLayers(strings.NewReader(""), os.DirFS(filepath.Join(dir, ".tt")), tt.Builtin), dir
}

// writeUserResource creates a user resource inside the directory returned by
// newTestLayers.
func writeUserResource(t *testing.T, dir, typ, name, content string) {
	t.Helper()

	path := filepath.Join(dir, ".tt", typ, name)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestIndex(t *testing.T) {
	t.Parallel()

	names := []string{"default", "gruvbox", "nord"}

	tests := map[string]int{
		"default":              0,
		"nord":                 2,
		"/home/me/.tt/gruvbox": 1,
		"missing":              -1,
		"/somewhere/missing":   -1,
	}

	for name, want := range tests {
		if got := Index(names, name); got != want {
			t.Errorf("Index(%q) = %d, want %d", name, got, want)
		}
	}
}

func TestRead(t *testing.T) {
	t.Parallel()

	res, dir := newTestLayers(t)

	if got, err := res.Read("words", "200en"); err != nil || !strings.HasPrefix(string(got), "the\n") {
		t.Errorf("builtin word list not found, got %.20q, %v", got, err)
	}

	if got, err := res.Read("words", "does-not-exist"); err != ErrNotFound {
		t.Errorf("reading a missing resource = %q, %v, want %v", got, err, ErrNotFound)
	}

	writeUserResource(t, dir, "words", "200en", "overridden")
	if got, _ := res.Read("words", "200en"); string(got) != "overridden" {
		t.Errorf("user resource should take precedence over the builtin one, got %.20q", got)
	}

	path := filepath.Join(t.TempDir(), "custom")
	if err := os.WriteFile(path, []byte("from path"), 0600); err != nil {
		t.Fatal(err)
	}
	if got, _ := res.Read("words", path); string(got) != "from path" {
		t.Errorf("read(path) = %q, want %q", got, "from path")
	}
}

// The directories of resources are searched in order.
func TestLayers(t *testing.T) {
	t.Parallel()

	res := &Layers{
		dirs: []fs.FS{
			fstest.MapFS{"words/mine": {Data: []byte("first")}, "words/sub/x": {Data: []byte("in a directory")}},
			fstest.MapFS{"words/mine": {Data: []byte("second")}, "words/theirs": {Data: []byte("theirs")}, "themes/t": {}},
		},
		readFile: func(string) ([]byte, error) { return nil, os.ErrNotExist },
	}

	if got, _ := res.Read("words", "mine"); string(got) != "first" {
		t.Errorf("read(mine) = %q, want the first one", got)
	}
	if got, _ := res.Read("words", "theirs"); string(got) != "theirs" {
		t.Errorf("read(theirs) = %q", got)
	}
	if got := res.List("words"); !reflect.DeepEqual(got, []string{"mine", "theirs"}) {
		t.Errorf("list(words) = %q, want the files of every directory", got)
	}
}

func TestList(t *testing.T) {
	t.Parallel()

	res, dir := newTestLayers(t)
	writeUserResource(t, dir, "themes", "zzz-mine", "")
	writeUserResource(t, dir, "themes", "default", "")

	names := res.List("themes")
	if !sort.StringsAreSorted(names) {
		t.Errorf("theme names are not sorted: %v", names)
	}

	seen := map[string]int{}
	for _, n := range names {
		seen[n]++
	}

	for _, want := range []string{"default", "gruvbox", "_base", "zzz-mine"} {
		if seen[want] != 1 {
			t.Errorf("%q appears %d times in %v, want once", want, seen[want], names)
		}
	}

	words := res.List("words")
	for _, want := range []string{"1000en", "200en", "1000it", "it", "1000ru", "ru"} {
		if Index(words, want) == -1 {
			t.Errorf("word list %q missing from %v", want, words)
		}
	}

	if quotes := res.List("quotes"); !reflect.DeepEqual(quotes, []string{"en", "it"}) {
		t.Errorf("res.List(quotes) = %v, want [en it]", quotes)
	}
}

func TestReadStdinMoreThanOnce(t *testing.T) {
	t.Parallel()

	res := New(t.TempDir(), strings.NewReader("alpha beta"))

	for i := 0; i < 2; i++ {
		if got, err := res.Read("words", "-"); err != nil || string(got) != "alpha beta" {
			t.Errorf("reading - a %d time = %q, want %q", i+1, got, "alpha beta")
		}
	}
}
