package main

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gdamore/tcell"
)

func TestGetParagraphs(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"one", []string{"one"}},
		{"one\ntwo", []string{"one\ntwo"}},
		{"one\n\ntwo", []string{"one", "two"}},
		{"\n\none\r\n\r\n\r\n\ntwo\nthree\n\n", []string{"one", "two\nthree"}},
		{"one\n\n  \t\n\ntwo", []string{"one", "two"}},
		{"", nil},
		{" \n\n\t\n", nil},
	}

	for _, tc := range tests {
		if got := getParagraphs(tc.in); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("getParagraphs(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestWordWrap(t *testing.T) {
	tests := []struct {
		in   string
		n    int
		want string
	}{
		{"the quick brown fox jumps", 10, "the quick\nbrown fox\njumps"},
		{"the quick brown fox jumps", 80, "the quick brown fox jumps"},
		{"one\ntwo three", 80, "one two three"},
		{"averyveryverylongword short", 5, "averyveryverylongword\nshort"},
	}

	for _, tc := range tests {
		if got := wordWrap(tc.in, tc.n); got != tc.want {
			t.Errorf("wordWrap(%q, %d) = %q, want %q", tc.in, tc.n, got, tc.want)
		}
	}
}

func TestCalcStringDimensions(t *testing.T) {
	tests := []struct {
		in     string
		nc, nr int
	}{
		{"", 0, 0},
		{"abc", 3, 1},
		{"ab\ncde\nf", 3, 3},
		{"abc\n", 3, 2},
		{"àèì", 3, 1},
		{"\tab", tabWidth + 2, 1},
		{"a\n\t\tb", 2*tabWidth + 1, 2},
	}

	for _, tc := range tests {
		if nc, nr := calcStringDimensions(tc.in); nc != tc.nc || nr != tc.nr {
			t.Errorf("calcStringDimensions(%q) = (%d, %d), want (%d, %d)", tc.in, nc, nr, tc.nc, tc.nr)
		}
	}
}

func TestIndexResourceName(t *testing.T) {
	names := []string{"default", "gruvbox", "nord"}

	tests := map[string]int{
		"default":              0,
		"nord":                 2,
		"/home/me/.tt/gruvbox": 1,
		"missing":              -1,
		"/somewhere/missing":   -1,
	}

	for name, want := range tests {
		if got := indexResourceName(names, name); got != want {
			t.Errorf("indexResourceName(%q) = %d, want %d", name, got, want)
		}
	}
}

func TestReadResource(t *testing.T) {
	res, dir := newTestResources(t)

	if got, err := res.read("words", "200en"); err != nil || !strings.HasPrefix(string(got), "the\n") {
		t.Errorf("builtin word list not found, got %.20q, %v", got, err)
	}

	if got, err := res.read("words", "does-not-exist"); err != errNoResource {
		t.Errorf("reading a missing resource = %q, %v, want %v", got, err, errNoResource)
	}

	writeUserResource(t, dir, "words", "200en", "overridden")
	if got, _ := res.read("words", "200en"); string(got) != "overridden" {
		t.Errorf("user resource should take precedence over the builtin one, got %.20q", got)
	}

	path := filepath.Join(t.TempDir(), "custom")
	if err := os.WriteFile(path, []byte("from path"), 0600); err != nil {
		t.Fatal(err)
	}
	if got, _ := res.read("words", path); string(got) != "from path" {
		t.Errorf("read(path) = %q, want %q", got, "from path")
	}
}

// The directories of resources are searched in order.
func TestResourceDirectories(t *testing.T) {
	res := &resources{
		dirs: []fs.FS{
			fstest.MapFS{"words/mine": {Data: []byte("first")}, "words/sub/x": {Data: []byte("in a directory")}},
			fstest.MapFS{"words/mine": {Data: []byte("second")}, "words/theirs": {Data: []byte("theirs")}, "themes/t": {}},
		},
		readFile: func(string) ([]byte, error) { return nil, os.ErrNotExist },
	}

	if got, _ := res.read("words", "mine"); string(got) != "first" {
		t.Errorf("read(mine) = %q, want the first one", got)
	}
	if got, _ := res.read("words", "theirs"); string(got) != "theirs" {
		t.Errorf("read(theirs) = %q", got)
	}
	if got := res.list("words"); !reflect.DeepEqual(got, []string{"mine", "theirs"}) {
		t.Errorf("list(words) = %q, want the files of every directory", got)
	}
}

func TestListResourceNames(t *testing.T) {
	res, dir := newTestResources(t)
	writeUserResource(t, dir, "themes", "zzz-mine", "")
	writeUserResource(t, dir, "themes", "default", "")

	names := res.list("themes")
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

	words := res.list("words")
	for _, want := range []string{"1000en", "200en", "1000it", "it", "1000ru", "ru"} {
		if indexResourceName(words, want) == -1 {
			t.Errorf("word list %q missing from %v", want, words)
		}
	}

	if quotes := res.list("quotes"); !reflect.DeepEqual(quotes, []string{"en", "it"}) {
		t.Errorf("res.list(quotes) = %v, want [en it]", quotes)
	}
}

func TestDrawString(t *testing.T) {
	sim := newSimScreen(t)
	sim.SetSize(20, 5)

	drawString(sim, 2, 1, "ab\ncd", tcell.StyleDefault)

	if got := screenLine(sim, 1); got != "  ab" {
		t.Errorf("line 1 = %q, want %q", got, "  ab")
	}
	if got := screenLine(sim, 2); got != "  cd" {
		t.Errorf("line 2 = %q, want %q", got, "  cd")
	}

	sim.Clear()
	drawStringAtCenter(sim, "xyz", tcell.StyleDefault)
	if got := screenLine(sim, 2); got != "        xyz" {
		t.Errorf("centered line = %q, want %q", got, "        xyz")
	}
}

func TestWordWrapCountsCharacters(t *testing.T) {
	tests := []struct {
		in   string
		n    int
		want string
	}{
		{"абв где жзи", 7, "абв где\nжзи"},
		{"città più già", 9, "città più\ngià"},
	}

	for _, tc := range tests {
		if got := wordWrap(tc.in, tc.n); got != tc.want {
			t.Errorf("wordWrap(%q, %d) = %q, want %q", tc.in, tc.n, got, tc.want)
		}
	}
}

func TestReflow(t *testing.T) {
	tests := []struct {
		in    string
		width int
		want  string
	}{
		{"alpha\nbeta   gamma\n", 80, "alpha beta gamma"},
		{"  the quick brown fox ", 10, "the quick \nbrown fox"},
		{"the quick \nbrown fox", 80, "the quick brown fox"},
	}

	for _, tc := range tests {
		got := reflow(tc.in, tc.width)
		if got != tc.want {
			t.Errorf("reflow(%q, %d) = %q, want %q", tc.in, tc.width, got, tc.want)
		}

		// Tests are reflowed again whenever they are shown.
		if again := reflow(got, tc.width); again != got {
			t.Errorf("reflow(%q, %d) = %q, want it unchanged", got, tc.width, again)
		}
	}
}

func TestLineWidth(t *testing.T) {
	tests := []struct{ maxLineLen, sw, want int }{
		{80, 120, 80},
		{80, 80, 80},
		{80, 50, 42},
		{20, 50, 20},
	}

	for _, tc := range tests {
		if got := lineWidth(tc.maxLineLen, tc.sw); got != tc.want {
			t.Errorf("lineWidth(%d, %d) = %d, want %d", tc.maxLineLen, tc.sw, got, tc.want)
		}
	}
}

func TestBuiltinResourcesMatchFiles(t *testing.T) {
	res, _ := newTestResources(t)

	for _, typ := range []string{"themes", "words", "quotes"} {
		dir := filepath.Join("..", "..", typ)
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}

		if names := res.list(typ); len(names) != len(entries) {
			t.Errorf("%d builtin %s, want %d", len(names), typ, len(entries))
		}

		for _, e := range entries {
			want, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				t.Fatal(err)
			}

			if got, _ := res.read(typ, e.Name()); !bytes.Equal(got, want) {
				t.Errorf("builtin %s/%s differs from the file in the repository", typ, e.Name())
			}
		}
	}
}

func TestReadResourceFromStdinMoreThanOnce(t *testing.T) {
	res := newResources(t.TempDir(), strings.NewReader("alpha beta"))

	for i := 0; i < 2; i++ {
		if got, err := res.read("words", "-"); err != nil || string(got) != "alpha beta" {
			t.Errorf("reading - a %d time = %q, want %q", i+1, got, "alpha beta")
		}
	}
}
