package main

import (
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/gdamore/tcell"
)

var configDirs []string

func init() {
	home, _ := os.LookupEnv("HOME")

	configDirs = []string{
		filepath.Join(home, ".tt"),
		"/etc/tt",
	}
}

func getParagraphs(s string) []string {
	s = strings.Replace(s, "\r", "", -1)
	s = regexp.MustCompile("\n\n+").ReplaceAllString(s, "\n\n")
	return strings.Split(strings.Trim(s, "\n"), "\n\n")
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

func randomText(n int, words []string) string {
	r := ""

	//Consecutive repetitions can only be avoided if there are at least two distinct words.
	canAvoidRepeats := false
	for _, w := range words {
		if w != words[0] {
			canAvoidRepeats = true
			break
		}
	}

	var last string
	for i := 0; i < n; i++ {
		w := words[rand.Int()%len(words)]
		for canAvoidRepeats && last == w {
			w = words[rand.Int()%len(words)]
		}

		r += w
		if i != n-1 {
			r += " "
		}

		last = w
	}

	return strings.Replace(r, "\n", " \n", -1)
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
			c++
		}
	}

	nr++
	if c > nc {
		nc = c
	}

	return
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
