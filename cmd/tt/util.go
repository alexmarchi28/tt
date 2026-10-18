package main

import (
	"regexp"
	"sort"
	"strings"

	"github.com/gdamore/tcell"
	"github.com/mattn/go-runewidth"
)

// blankLines matches the blank lines separating paragraphs.
var blankLines = regexp.MustCompile("\n\n+")

// getParagraphs returns the paragraphs of s which aren't blank.
func getParagraphs(s string) []string {
	s = strings.Replace(s, "\r", "", -1)
	s = blankLines.ReplaceAllString(s, "\n\n")

	var paragraphs []string
	for _, p := range strings.Split(strings.Trim(s, "\n"), "\n\n") {
		if strings.TrimSpace(p) != "" {
			paragraphs = append(paragraphs, p)
		}
	}

	return paragraphs
}

func wordWrapRunes(s []rune, n int) {
	sp := 0
	sz := 0

	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			s[i] = ' '
		}

		sz += cellWidth(s[i])

		if s[i] == ' ' {
			sp = i
		}

		if sz > n {
			if sp != 0 {
				s[sp] = '\n'
			}

			sz = runesWidth(s[sp+1 : i+1])
		}
	}

}

var whitespace = regexp.MustCompile(`\s+`)

// reflow joins the lines of s and wraps it to the given width. Line breaks
// are preceded by a space so that they can be typed like any other word
// separator.
func reflow(s string, width int) string {
	s = whitespace.ReplaceAllString(s, " ")
	return strings.ReplaceAll(wordWrap(strings.Trim(s, " "), width), "\n", " \n")
}

// lineWidth returns the width at which text is wrapped on a screen sw cells
// wide.
func lineWidth(maxLineLen, sw int) int {
	if maxLineLen > sw {
		maxLineLen = sw - 8
		if maxLineLen < 1 {
			maxLineLen = sw //Too narrow for a margin.
		}
	}

	if maxLineLen < 1 {
		return 1
	}

	return maxLineLen
}

func wordWrap(s string, n int) string {
	r := []rune(s)
	wordWrapRunes(r, n)
	return string(r)
}

func drawString(scr tcell.Screen, x, y int, s string, style tcell.Style) {
	sx := x

	for _, c := range s {
		if c == '\n' {
			y++
			x = sx
		} else {
			x += drawRune(scr, x, y, c, style)
		}
	}
}

func drawStringAtCenter(scr tcell.Screen, s string, style tcell.Style) {
	nc, nr := calcStringDimensions(s)
	sw, sh := scr.Size()

	x := (sw - nc) / 2
	y := (sh - nr) / 2

	drawString(scr, x, y, s, style)
}

// tabWidth is the number of cells taken by a tab.
const tabWidth = 4

// cellWidth returns the number of cells taken by r on the screen.
func cellWidth(r rune) int {
	if r == '\t' {
		return tabWidth
	}

	if w := runewidth.RuneWidth(r); w > 0 {
		return w
	}

	return 1 //Control and combining characters still take a cell.
}

// runesWidth returns the number of cells taken by s.
func runesWidth(s []rune) int {
	w := 0
	for _, r := range s {
		w += cellWidth(r)
	}

	return w
}

// drawRune draws r at (x, y) and returns the number of cells it takes. A tab
// is drawn as spaces, and a wide character once, the screen spanning it over
// the following cell.
func drawRune(scr tcell.Screen, x, y int, r rune, style tcell.Style) int {
	w := cellWidth(r)
	if r == '\t' {
		for i := 0; i < w; i++ {
			scr.SetContent(x+i, y, ' ', nil, style)
		}
	} else {
		scr.SetContent(x, y, r, nil, style)
	}

	return w
}

func calcStringDimensions(s string) (nc, nr int) {
	if s == "" {
		return 0, 0
	}

	c := 0

	for _, x := range s {
		if x == '\n' {
			nr++
			if c > nc {
				nc = c
			}
			c = 0
		} else {
			c += cellWidth(x)
		}
	}

	nr++
	if c > nc {
		nc = c
	}

	return
}

// sortedKeys returns the keys of m in alphabetical order.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	return keys
}

// contains reports whether list contains s.
func contains(list []string, s string) bool {
	for _, candidate := range list {
		if candidate == s {
			return true
		}
	}

	return false
}
