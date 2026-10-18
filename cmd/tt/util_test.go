package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/gdamore/tcell"
)

func TestGetParagraphs(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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

func TestDrawString(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	tests := []struct {
		in   string
		n    int
		want string
	}{
		{"абв где жзи", 7, "абв где\nжзи"},
		{"città più già", 9, "città più\ngià"},
		//Wide characters take two cells.
		{"日本語 日本語", 7, "日本語\n日本語"},
		{"日本 ab", 7, "日本 ab"},
	}

	for _, tc := range tests {
		if got := wordWrap(tc.in, tc.n); got != tc.want {
			t.Errorf("wordWrap(%q, %d) = %q, want %q", tc.in, tc.n, got, tc.want)
		}
	}
}

func TestReflow(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	tests := []struct{ maxLineLen, sw, want int }{
		{80, 120, 80},
		{80, 80, 80},
		{80, 50, 42},
		{20, 50, 20},
		//Very narrow screens leave no margin.
		{80, 8, 8},
		{80, 3, 3},
		{80, 0, 1},
	}

	for _, tc := range tests {
		if got := lineWidth(tc.maxLineLen, tc.sw); got != tc.want {
			t.Errorf("lineWidth(%d, %d) = %d, want %d", tc.maxLineLen, tc.sw, got, tc.want)
		}
	}
}

func TestBuiltinResourcesMatchFiles(t *testing.T) {
	t.Parallel()

	res, _ := newTestResources(t)

	for _, typ := range []string{"themes", "words", "quotes"} {
		dir := filepath.Join("..", "..", typ)
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}

		if names := res.List(typ); len(names) != len(entries) {
			t.Errorf("%d builtin %s, want %d", len(names), typ, len(entries))
		}

		for _, e := range entries {
			want, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				t.Fatal(err)
			}

			if got, _ := res.Read(typ, e.Name()); !bytes.Equal(got, want) {
				t.Errorf("builtin %s/%s differs from the file in the repository", typ, e.Name())
			}
		}
	}
}

func TestCellWidth(t *testing.T) {
	t.Parallel()

	for r, want := range map[rune]int{'a': 1, 'è': 1, 'ж': 1, '\t': tabWidth, '日': 2, '한': 2} {
		if got := cellWidth(r); got != want {
			t.Errorf("cellWidth(%q) = %d, want %d", r, got, want)
		}
	}

	if nc, nr := calcStringDimensions("日本\nab"); nc != 4 || nr != 2 {
		t.Errorf("calcStringDimensions = %d, %d, want 4 cells and 2 rows", nc, nr)
	}
}

func TestDrawStringWithWideCharactersAndTabs(t *testing.T) {
	t.Parallel()

	sim := newSimScreen(t)
	sim.SetSize(20, 5)

	drawString(sim, 0, 0, "日本\tx", tcell.StyleDefault)

	if got := screenLine(sim, 0); got != "日本    x" {
		t.Errorf("line = %q, want the wide characters, the tab as spaces then x", got)
	}
}
