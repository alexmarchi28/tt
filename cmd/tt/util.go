package main

import (
	"regexp"
	"sort"
	"strings"

	"github.com/gdamore/tcell"
)

// getParagraphs returns the paragraphs of s which aren't blank.
func getParagraphs(s string) []string {
	s = strings.Replace(s, "\r", "", -1)
	s = regexp.MustCompile("\n\n+").ReplaceAllString(s, "\n\n")

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
		sz++

		if s[i] == '\n' {
			s[i] = ' '
		}

		if s[i] == ' ' {
			sp = i
		}

		if sz > n {
			if sp != 0 {
				s[sp] = '\n'
			}

			sz = i - sp
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
		return sw - 8
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
			scr.SetContent(x, y, c, nil, style)
			x++
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

	return 1
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
