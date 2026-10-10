package main

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

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
	}

	for _, tc := range tests {
		if nc, nr := calcStringDimensions(tc.in); nc != tc.nc || nr != tc.nr {
			t.Errorf("calcStringDimensions(%q) = (%d, %d), want (%d, %d)", tc.in, nc, nr, tc.nc, tc.nr)
		}
	}
}

func TestRandomText(t *testing.T) {
	words := []string{"alpha", "beta", "gamma"}

	for i := 0; i < 50; i++ {
		got := strings.Split(randomText(20, words), " ")
		if len(got) != 20 {
			t.Fatalf("randomText produced %d words, want 20: %q", len(got), got)
		}

		for j, w := range got {
			if w != "alpha" && w != "beta" && w != "gamma" {
				t.Fatalf("unexpected word %q in %q", w, got)
			}
			if j > 0 && got[j-1] == w {
				t.Fatalf("repeated word %q in %q", w, got)
			}
		}
	}

	if got := randomText(1, []string{"two\nlines"}); got != "two \nlines" {
		t.Errorf("randomText with an embedded newline = %q, want %q", got, "two \nlines")
	}
}

func TestExtractMistypedWords(t *testing.T) {
	tests := []struct {
		text, typed string
		want        []mistake
	}{
		{"alpha beta", "alpha beta", nil},
		{"alpha beta", "alpha betx", []mistake{{"beta", "betx"}}},
		{"alpha beta", "alxha bxta", []mistake{{"alpha", "alxha"}, {"beta", "bxta"}}},
		{"ab cd", "a\x00 cd", []mistake{{"ab", "a_"}}},
		{"ab cd", "ab cd"[:4] + "\x00", []mistake{{"cd", "c_"}}},
	}

	for _, tc := range tests {
		got := extractMistypedWords([]rune(tc.text), []rune(tc.typed))
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("extractMistypedWords(%q, %q) = %v, want %v", tc.text, tc.typed, got, tc.want)
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

func TestExtractMistypedWordsAcrossLines(t *testing.T) {
	tests := []struct {
		text, typed string
		want        []mistake
	}{
		{"ab \ncd", "ab \ncx", []mistake{{"cd", "cx"}}},
		{"ab\ncd", "ab\ncx", []mistake{{"cd", "cx"}}},
		{"ab\ncd", "xb\ncd", []mistake{{"ab", "xb"}}},
	}

	for _, tc := range tests {
		got := extractMistypedWords([]rune(tc.text), []rune(tc.typed))
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("extractMistypedWords(%q, %q) = %q, want %q", tc.text, tc.typed, got, tc.want)
		}
	}
}

func TestRandomTextWithoutAlternatives(t *testing.T) {
	tests := map[string][]string{
		"single word":     {"solo"},
		"same word twice": {"solo", "solo"},
	}

	for name, words := range tests {
		t.Run(name, func(t *testing.T) {
			done := make(chan string, 1)
			go func() { done <- randomText(4, words) }()

			select {
			case got := <-done:
				if got != "solo solo solo solo" {
					t.Errorf("randomText = %q, want %q", got, "solo solo solo solo")
				}
			case <-time.After(time.Second):
				t.Fatal("randomText did not return")
			}
		})
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
