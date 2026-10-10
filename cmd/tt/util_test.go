package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

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
	dir := useTempStorage(t)

	if got := readResource("words", "200en"); !strings.HasPrefix(string(got), "the\n") {
		t.Errorf("builtin word list not found, got %.20q", got)
	}

	if got := readResource("words", "does-not-exist"); got != nil {
		t.Errorf("readResource of a missing resource = %q, want nil", got)
	}

	writeUserResource(t, dir, "words", "200en", "overridden")
	if got := readResource("words", "200en"); string(got) != "overridden" {
		t.Errorf("user resource should take precedence over the builtin one, got %.20q", got)
	}

	path := filepath.Join(t.TempDir(), "custom")
	if err := os.WriteFile(path, []byte("from path"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := readResource("words", path); string(got) != "from path" {
		t.Errorf("readResource(path) = %q, want %q", got, "from path")
	}
}

func TestListResourceNames(t *testing.T) {
	dir := useTempStorage(t)
	writeUserResource(t, dir, "themes", "zzz-mine", "")
	writeUserResource(t, dir, "themes", "default", "")

	names := listResourceNames("themes")
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

	words := listResourceNames("words")
	for _, want := range []string{"1000en", "200en", "it"} {
		if indexResourceName(words, want) == -1 {
			t.Errorf("word list %q missing from %v", want, words)
		}
	}

	if quotes := listResourceNames("quotes"); !reflect.DeepEqual(quotes, []string{"en"}) {
		t.Errorf("listResourceNames(quotes) = %v, want [en]", quotes)
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
	useTempStorage(t)

	for _, typ := range []string{"themes", "words", "quotes"} {
		dir := filepath.Join("..", "..", typ)
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}

		if names := listResourceNames(typ); len(names) != len(entries) {
			t.Errorf("%d builtin %s, want %d", len(names), typ, len(entries))
		}

		for _, e := range entries {
			want, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				t.Fatal(err)
			}

			if got := readResource(typ, e.Name()); !bytes.Equal(got, want) {
				t.Errorf("builtin %s/%s differs from the file in the repository", typ, e.Name())
			}
		}
	}
}
