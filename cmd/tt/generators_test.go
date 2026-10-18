package main

import (
	"encoding/json"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode"
	"unicode/utf8"
)

func TestGenerateTestFromDataRaw(t *testing.T) {
	gen := mustGen(generateTestFromData([]byte("one\n\ntwo  three\n"), true, false))

	for i := 0; i < 2; i++ {
		want := []segment{{"one\n\ntwo  three\n", ""}}
		if got := gen(); !reflect.DeepEqual(got, want) {
			t.Fatalf("call %d: got %q, want %q", i, got, want)
		}
	}
}

func TestGenerateTestFromDataMulti(t *testing.T) {
	gen := mustGen(generateTestFromData([]byte("one\n\ntwo\nthree\n"), false, true))

	for _, want := range [][]segment{{{"one", ""}}, {{"two\nthree", ""}}, nil, nil} {
		if got := gen(); !reflect.DeepEqual(got, want) {
			t.Fatalf("got %q, want %q", got, want)
		}
	}
}

func TestGenerateTestFromDataParagraphs(t *testing.T) {
	gen := mustGen(generateTestFromData([]byte("one\n\ntwo\nthree\n"), false, false))

	want := []segment{{"one", ""}, {"two\nthree", ""}}
	for i := 0; i < 2; i++ {
		if got := gen(); !reflect.DeepEqual(got, want) {
			t.Fatalf("call %d: got %q, want %q", i, got, want)
		}
	}
}

func TestGenerateWordTest(t *testing.T) {
	res, dir := newTestResources(t)
	writeUserResource(t, dir, "words", "greek", "alpha beta\tgamma\ndelta")

	gen := mustGen(generateWordTest(newRand(), res, "greek", wordOptions{n: 3, g: 2}))
	allowed := map[string]bool{"alpha": true, "beta": true, "gamma": true, "delta": true}

	for i := 0; i < 20; i++ {
		segments := gen()
		if len(segments) != 2 {
			t.Fatalf("got %d segments, want 2", len(segments))
		}

		for _, s := range segments {
			if s.Attribution != "" {
				t.Errorf("unexpected attribution %q", s.Attribution)
			}

			words := strings.Split(s.Text, " ")
			if len(words) != 3 {
				t.Fatalf("segment %q has %d words, want 3", s.Text, len(words))
			}
			for _, w := range words {
				if !allowed[w] {
					t.Fatalf("unexpected word %q in %q", w, s.Text)
				}
			}
		}
	}
}

func TestGenerateWordTestUsesBuiltinLists(t *testing.T) {
	res, _ := newTestResources(t)

	gen := mustGen(generateWordTest(newRand(), res, "1000en", wordOptions{n: 50, g: 1}))
	for i := 0; i < 100; i++ {
		segments := gen()
		if len(segments) != 1 {
			t.Fatalf("got %d segments, want 1", len(segments))
		}
		assertWords(t, segments[0].Text, 50)
	}
}

func TestGenerateWordTestIgnoresBlankEntries(t *testing.T) {
	res, dir := newTestResources(t)
	writeUserResource(t, dir, "words", "blanks", "\n  alpha beta\n\ngamma\t\n")

	gen := mustGen(generateWordTest(newRand(), res, "blanks", wordOptions{n: 10, g: 1}))
	for i := 0; i < 100; i++ {
		assertWords(t, gen()[0].Text, 10)
	}
}

// assertWords checks that text consists of n words separated by single
// spaces.
func assertWords(t *testing.T, text string, n int) {
	t.Helper()

	words := strings.Split(text, " ")
	if len(words) != n {
		t.Fatalf("%q has %d words, want %d", text, len(words), n)
	}
	for _, w := range words {
		if w == "" {
			t.Fatalf("%q contains an empty word", text)
		}
	}
}

func TestGenerateQuoteTest(t *testing.T) {
	res, dir := newTestResources(t)
	writeUserResource(t, dir, "quotes", "mine", `[
		{"text": "first quote", "attribution": "someone"},
		{"text": "second quote", "attribution": "someone else"}
	]`)

	gen := mustGen(generateQuoteTest(newRand(), res, "mine"))
	seen := map[string]bool{}

	for i := 0; i < 50; i++ {
		got := gen()
		if len(got) != 1 {
			t.Fatalf("got %d segments, want 1", len(got))
		}

		switch got[0] {
		case segment{"first quote", "someone"}, segment{"second quote", "someone else"}:
			seen[got[0].Text] = true
		default:
			t.Fatalf("unexpected quote %q", got[0])
		}
	}

	if len(seen) != 2 {
		t.Errorf("only saw quotes %v in 50 draws", seen)
	}
}

func TestBuiltinQuotesAreValid(t *testing.T) {
	res, _ := newTestResources(t)

	names := res.list("quotes")
	for _, want := range []string{"en", "it"} {
		if indexResourceName(names, want) == -1 {
			t.Errorf("no builtin %s quotes in %q", want, names)
		}
	}

	for _, name := range names {
		var quotes []segment
		b, err := res.read("quotes", name)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(b, &quotes); err != nil {
			t.Fatalf("builtin quotes %s: %v", name, err)
		}

		for _, q := range quotes {
			if strings.TrimSpace(q.Text) == "" {
				t.Errorf("builtin quotes %s: %+v has no text", name, q)
			}
		}
	}
}

// The builtin word lists whose words are checked: made of the given letters,
// distinct and, if given, as many as the size.
var checkedWordLists = []struct {
	name    string
	letters string
	size    int
}{
	{"1000it", "abcdefghijklmnopqrstuvwxyzàèéìòù", 1000},
	{"ru", "абвгдеёжзийклмнопрстуфхцчшщъыьэюя", 0},
	{"1000ru", "абвгдеёжзийклмнопрстуфхцчшщъыьэюя", 1000},
}

func TestBuiltinWordLists(t *testing.T) {
	res, _ := newTestResources(t)

	for _, list := range checkedWordLists {
		words, err := loadWords(res, list.name)
		if err != nil {
			t.Fatal(err)
		}

		seen := map[string]bool{}
		for _, w := range words {
			if seen[w] {
				t.Errorf("%s: %q appears twice", list.name, w)
			}
			seen[w] = true

			if strings.Trim(w, list.letters) != "" {
				t.Errorf("%s: %q is not made of lower case letters of the language", list.name, w)
			}
		}

		if list.size > 0 && len(seen) != list.size {
			t.Errorf("%s: got %d distinct words, want %d", list.name, len(seen), list.size)
		}
	}
}

func writeTextFile(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "book.txt")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	return path
}

func assertNextParagraph(t *testing.T, gen func() []segment, want string) {
	t.Helper()

	got := gen()
	if want == "" {
		if got != nil {
			t.Fatalf("got %q, want the end of the file", got)
		}
		return
	}

	if !reflect.DeepEqual(got, []segment{{want, ""}}) {
		t.Fatalf("got %q, want paragraph %q", got, want)
	}
}

func TestGenerateTestFromFile(t *testing.T) {
	st := &memStore{}
	path := writeTextFile(t, "p0\n\np1\nstill p1\n\np2\n")

	gen := nextFunc(t, mustSource(newFileSource(path, -1, st)))
	assertNextParagraph(t, gen, "p0")
	assertNextParagraph(t, gen, "p1\nstill p1")
	assertNextParagraph(t, gen, "p2")
	assertNextParagraph(t, gen, "")
}

func TestGenerateTestFromFileRemembersPosition(t *testing.T) {
	st := &memStore{}
	path := writeTextFile(t, "p0\n\np1\n\np2\n")

	gen := nextFunc(t, mustSource(newFileSource(path, -1, st)))
	assertNextParagraph(t, gen, "p0")
	assertNextParagraph(t, gen, "p1")

	// A new run resumes from the paragraph that was last shown.
	gen = nextFunc(t, mustSource(newFileSource(path, -1, st)))
	assertNextParagraph(t, gen, "p1")
	assertNextParagraph(t, gen, "p2")
	assertNextParagraph(t, gen, "")

	gen = nextFunc(t, mustSource(newFileSource(path, -1, st)))
	assertNextParagraph(t, gen, "")

	// -start overrides (and resets) the stored position.
	gen = nextFunc(t, mustSource(newFileSource(path, 0, st)))
	assertNextParagraph(t, gen, "p0")

	gen = nextFunc(t, mustSource(newFileSource(path, 2, st)))
	assertNextParagraph(t, gen, "p2")
}

func TestGenerateTestFromFileTracksFilesSeparately(t *testing.T) {
	st := &memStore{}
	a := writeTextFile(t, "a0\n\na1\n")
	b := writeTextFile(t, "b0\n\nb1\n")

	assertNextParagraph(t, nextFunc(t, mustSource(newFileSource(a, 1, st))), "a1")
	assertNextParagraph(t, nextFunc(t, mustSource(newFileSource(b, -1, st))), "b0")
	assertNextParagraph(t, nextFunc(t, mustSource(newFileSource(a, -1, st))), "a1")
}

func TestNewTestGenerator(t *testing.T) {
	res, _ := newTestResources(t)
	book := writeTextFile(t, "first paragraph\n\nsecond paragraph\n")
	st := mistakesOf("ab")

	tests := []struct {
		name            string
		args            []string
		stdin           string
		stdinIsTerminal bool
		check           func(t *testing.T, segments []segment)
		kind            testKind
	}{
		{"default", nil, "", true, wordsPerSegment(50), testKind{mode: "words", source: "1000en", words: 50}},
		{"word list", []string{"-words", "200en", "-n", "3", "-g", "2"}, "", true, wordsPerSegment(3, 3), testKind{mode: "words", source: "200en", words: 6}},
		{"quotes", []string{"-quotes", "en"}, "", true, func(t *testing.T, segments []segment) {
			if len(segments) != 1 || segments[0].Text == "" {
				t.Errorf("got %q, want a quote", segments)
			}
		}, testKind{mode: "quotes", source: "en"}},
		{"stdin", nil, "piped text\n\nmore", false, exactly("piped text", "more"), testKind{mode: "text"}},
		{"word list over stdin", []string{"-words", "200en", "-n", "4"}, "piped text", false, wordsPerSegment(4), testKind{mode: "words", source: "200en", words: 4}},
		{"file", []string{book}, "", true, exactly("first paragraph"), testKind{mode: "file", source: "book.txt"}},
		{"stdin over file", []string{book}, "piped text", false, exactly("piped text"), testKind{mode: "text"}},
		{"mistakes", []string{"-mistakes", "-n", "3"}, "", true, exactly("ab ab ab"), testKind{mode: "mistakes", words: 3}},
		{"punctuation and numbers", []string{"-punct", "-numbers", "-n", "200"}, "", true, func(t *testing.T, segments []segment) {
			if len(segments) != 1 || !endsSentence(segments[0].Text) || !strings.ContainsAny(segments[0].Text, "0123456789") {
				t.Errorf("got %q, want sentences with numbers", segments)
			}
		}, testKind{mode: "words", source: "1000en", words: 200, punct: true, numbers: true}},
		{"mistakes over stdin", []string{"-mistakes", "-n", "2"}, "piped text", false, exactly("ab ab"), testKind{mode: "mistakes", words: 2}},
		{"adaptive", []string{"-adaptive", "-n", "5"}, "", true, wordsPerSegment(5), testKind{mode: "adaptive", source: "1000en", words: 5, focus: 40}},
		{"adaptive word list", []string{"-adaptive", "-words", "200en", "-n", "5", "-focus", "70"}, "piped text", false, wordsPerSegment(5), testKind{mode: "adaptive", source: "200en", words: 5, focus: 70}},
		{"focus without -adaptive", []string{"-n", "5", "-focus", "70"}, "", true, wordsPerSegment(5), testKind{mode: "words", source: "1000en", words: 5}},
		{"timed words", []string{"-t", "10", "-n", "5"}, "", true, lastsAtLeast(10*time.Second, 5), testKind{mode: "words", source: "1000en", seconds: 10}},
		{"timed mistakes", []string{"-mistakes", "-t", "10", "-n", "5"}, "", true, lastsAtLeast(10*time.Second, 5), testKind{mode: "mistakes", seconds: 10}},
		{"timed adaptive", []string{"-adaptive", "-t", "10", "-n", "5"}, "", true, lastsAtLeast(10*time.Second, 5), testKind{mode: "adaptive", source: "1000en", seconds: 10, focus: 40}},
		{"timed quotes", []string{"-quotes", "en", "-t", "30"}, "", true, func(t *testing.T, segments []segment) {
			if len(segments) != 1 || segments[0].Text == "" {
				t.Errorf("got %q, want a quote", segments)
			}
		}, testKind{mode: "quotes", source: "en", seconds: 30}},
		{"word options over stdin", []string{"-punct", "-numbers", "-n", "3"}, "piped text", false, exactly("piped text"), testKind{mode: "text"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gen, kind, err := newTestSource(parseFlags(tc.args), textSources{st, res, newRand(), strings.NewReader(tc.stdin), tc.stdinIsTerminal})
			if err != nil {
				t.Fatal(err)
			}
			tc.check(t, nextTest(t, gen))

			if kind != tc.kind {
				t.Errorf("kind = %+v, want %+v", kind, tc.kind)
			}
		})
	}
}

// wordsPerSegment checks that there is a segment of n words for each given n.
func wordsPerSegment(n ...int) func(t *testing.T, segments []segment) {
	return func(t *testing.T, segments []segment) {
		t.Helper()

		if len(segments) != len(n) {
			t.Fatalf("got %d segments, want %d", len(segments), len(n))
		}
		for i, s := range segments {
			assertWords(t, s.Text, n[i])
		}
	}
}

// lastsAtLeast checks that the segments, of n words each, are too long to be
// typed before the time limit even by a fast typist (300 wpm).
func lastsAtLeast(limit time.Duration, n int) func(t *testing.T, segments []segment) {
	return func(t *testing.T, segments []segment) {
		t.Helper()

		chars := 0
		for _, s := range segments {
			assertWords(t, s.Text, n)
			chars += utf8.RuneCountInString(s.Text)
		}

		if typed := int(limit.Minutes() * 300 * 5); chars < typed {
			t.Errorf("%d segments of %d characters, a typist at 300 wpm types %d in %v", len(segments), chars, typed, limit)
		}
	}
}

func exactly(texts ...string) func(t *testing.T, segments []segment) {
	return func(t *testing.T, segments []segment) {
		t.Helper()

		var want []segment
		for _, text := range texts {
			want = append(want, segment{text, ""})
		}
		if !reflect.DeepEqual(segments, want) {
			t.Errorf("got %q, want %q", segments, want)
		}
	}
}

// mistakesOf returns a store of mistakes in the given words.
func mistakesOf(words ...string) *memStore {
	var mistakes []mistake
	for _, w := range words {
		mistakes = append(mistakes, mistake{w, "x"})
	}

	return &memStore{mistakes: mistakes}
}

func TestWeightedMistakes(t *testing.T) {
	mistakes := []mistake{{"a", "x"}, {"b", "x"}, {"a", "x"}, {"c", "x"}, {"a", "x"}}

	got := weightedMistakes(mistakes, 10)
	want := []string{"a", "a", "a", "b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("weightedMistakes() = %q, want %q", got, want)
	}

	// Only the most recent mistakes count.
	if got := weightedMistakes(mistakes, 2); !reflect.DeepEqual(got, []string{"a", "c"}) {
		t.Errorf("weightedMistakes(2) = %q, want [a c]", got)
	}
}

func TestGenerateMistakesTest(t *testing.T) {
	var words []string
	for i := 0; i < 100; i++ {
		words = append(words, "old")
	}
	for i := 0; i < recentMistakes/10; i++ {
		words = append(words, "often", "often", "often", "often", "often", "often", "often", "often", "rarely", "seldom")
	}
	gen := mustGen(generateMistakesTest(newRand(), wordOptions{n: 50, g: 2}, mistakesOf(words...)))
	counts := map[string]int{}
	for i := 0; i < 50; i++ {
		segments := gen()
		if len(segments) != 2 {
			t.Fatalf("got %d segments, want 2", len(segments))
		}

		for _, s := range segments {
			assertWords(t, s.Text, 50)
			for _, w := range strings.Fields(s.Text) {
				counts[w]++
			}
		}
	}

	if counts["old"] != 0 {
		t.Errorf("%d words come from old mistakes", counts["old"])
	}

	// Without weights "often" would make a third of the words.
	if share := float64(counts["often"]) / 5000; share < 0.4 {
		t.Errorf("often makes %.2f of the words, want frequent mistakes to come up more often (%v)", share, counts)
	}
}

func TestRandomWords(t *testing.T) {
	words := []string{"alpha", "beta", "gamma"}
	rng := newRand()

	for i := 0; i < 50; i++ {
		got := randomWords(rng, 20, words)
		if len(got) != 20 {
			t.Fatalf("randomWords produced %d words, want 20: %q", len(got), got)
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
}

func TestRandomWordsWithoutAlternatives(t *testing.T) {
	tests := map[string][]string{
		"single word":     {"solo"},
		"same word twice": {"solo", "solo"},
	}

	for name, words := range tests {
		t.Run(name, func(t *testing.T) {
			done := make(chan []string, 1)
			go func() { done <- randomWords(newRand(), 4, words) }()

			select {
			case got := <-done:
				if want := []string{"solo", "solo", "solo", "solo"}; !reflect.DeepEqual(got, want) {
					t.Errorf("randomWords = %q, want %q", got, want)
				}
			case <-time.After(time.Second):
				t.Fatal("randomWords did not return")
			}
		})
	}
}

// startsUpper reports whether the first letter of w is upper case, ignoring
// opening quotes and parentheses.
func startsUpper(w string) bool {
	r, _ := utf8.DecodeRuneInString(strings.TrimLeft(w, `"(`))
	return unicode.IsUpper(r)
}

func endsSentence(w string) bool {
	return strings.ContainsAny(w[len(w)-1:], ".?!")
}

func TestPunctuate(t *testing.T) {
	words := []string{"alpha", "beta", "gamma", "delta", "épsilon", "zeta", "eta", "theta"}
	marks := map[string]int{}
	rng := newRand()

	for i := 0; i < 500; i++ {
		got := punctuate(rng, words)
		if len(got) != len(words) {
			t.Fatalf("punctuate() returned %d words, want %d", len(got), len(words))
		}

		if !startsUpper(got[0]) || !endsSentence(got[len(got)-1]) {
			t.Errorf("%q is not made of whole sentences", got)
		}

		for j, w := range got {
			bare := strings.Trim(w, `.,;:?!"()`)
			if !strings.EqualFold(bare, words[j]) {
				t.Fatalf("word %q became %q", words[j], w)
			}
			if j > 0 && startsUpper(w) != endsSentence(got[j-1]) {
				t.Errorf("%q: only words starting a sentence should be capitalized", got)
			}

			for _, m := range []string{".", ",", ";", ":", "?", "!", `"`, "("} {
				if strings.Contains(w, m) {
					marks[m]++
				}
			}
		}
	}

	for _, m := range []string{".", ",", ";", ":", "?", "!", `"`, "("} {
		if marks[m] == 0 {
			t.Errorf("%q never used: %v", m, marks)
		}
	}
	if marks[","] < marks[";"] || marks["."] < marks["?"] {
		t.Errorf("unrealistic punctuation frequencies: %v", marks)
	}
}

func TestAddNumbers(t *testing.T) {
	words := strings.Fields(strings.Repeat("word ", 20))
	numbers := 0
	rng := newRand()

	for i := 0; i < 200; i++ {
		got := addNumbers(rng, words)
		if len(got) != len(words) {
			t.Fatalf("addNumbers() returned %d words, want %d", len(got), len(words))
		}

		for _, w := range got {
			if w == "word" {
				continue
			}

			numbers++
			if _, err := strconv.Atoi(w); err != nil || len(w) > 4 {
				t.Errorf("%q is neither a word nor a number of at most 4 digits", w)
			}
		}
	}

	if share := float64(numbers) / 4000; share < 0.08 || share > 0.25 {
		t.Errorf("%.2f of the words are numbers, want some but not too many", share)
	}
}

func TestGenerateWordTestDecorations(t *testing.T) {
	res, dir := newTestResources(t)
	writeUserResource(t, dir, "words", "greek", "alpha beta gamma delta")

	gen := mustGen(generateWordTest(newRand(), res, "greek", wordOptions{n: 30, g: 2, punctuation: true, numbers: true}))
	digits := false

	for i := 0; i < 20; i++ {
		for _, s := range gen() {
			words := strings.Split(s.Text, " ")
			if len(words) != 30 {
				t.Fatalf("%q has %d words, want 30", s.Text, len(words))
			}
			if !endsSentence(s.Text) {
				t.Errorf("%q does not end a sentence", s.Text)
			}
			digits = digits || strings.ContainsAny(s.Text, "0123456789")
		}
	}

	if !digits {
		t.Error("no number in 40 segments")
	}
}

func TestGenerateWordTestMinChars(t *testing.T) {
	res, dir := newTestResources(t)
	writeUserResource(t, dir, "words", "greek", "alpha beta gamma delta")

	gen := mustGen(generateWordTest(newRand(), res, "greek", wordOptions{n: 3, g: 1, minChars: 100}))
	for i := 0; i < 20; i++ {
		segments := gen()

		chars := 0
		for _, s := range segments {
			assertWords(t, s.Text, 3)
			chars += utf8.RuneCountInString(s.Text)
		}

		// Groups are added until there are enough characters, and no more.
		last := utf8.RuneCountInString(segments[len(segments)-1].Text)
		if chars < 100 || chars-last >= 100 {
			t.Fatalf("%d segments of %d characters, want just enough groups for 100", len(segments), chars)
		}
	}

	// -g still sets the least number of groups.
	gen = mustGen(generateWordTest(newRand(), res, "greek", wordOptions{n: 3, g: 50, minChars: 100}))
	if segments := gen(); len(segments) != 50 {
		t.Errorf("got %d segments, want 50", len(segments))
	}
}

// mustGen returns the generator made by a generate function, failing on
// errors.
func mustGen(gen func() []segment, err error) func() []segment {
	if err != nil {
		panic(err)
	}

	return gen
}

func TestGeneratorsRejectEmptyText(t *testing.T) {
	res, dir := newTestResources(t)
	blankFile := writeTextFile(t, "  \n\n\t\n")
	writeUserResource(t, dir, "quotes", "blank", `[{"text": " "}, {"text": ""}]`)

	tests := []struct {
		name string
		gen  func() (func() []segment, error)
		want string
	}{
		{"raw", func() (func() []segment, error) { return generateTestFromData([]byte(" \n"), true, false) }, "there is no text to type"},
		{"paragraphs", func() (func() []segment, error) { return generateTestFromData([]byte(" \n\n "), false, false) }, "there is no text to type"},
		{"multi", func() (func() []segment, error) { return generateTestFromData(nil, false, true) }, "there is no text to type"},
		{"file", func() (func() []segment, error) { _, err := newFileSource(blankFile, -1, &memStore{}); return nil, err }, blankFile + " does not contain any text"},
		{"quotes", func() (func() []segment, error) { return generateQuoteTest(newRand(), res, "blank") }, "blank does not contain any quotes"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := tc.gen(); err == nil || err.Error() != tc.want {
				t.Errorf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestGenerateQuoteTestIgnoresBlankQuotes(t *testing.T) {
	res, dir := newTestResources(t)
	writeUserResource(t, dir, "quotes", "some-blank", `[{"text": "  "}, {"text": "real quote"}]`)

	gen := mustGen(generateQuoteTest(newRand(), res, "some-blank"))
	for i := 0; i < 20; i++ {
		if got := gen(); got[0].Text != "real quote" {
			t.Fatalf("got %q, want the only quote with text", got)
		}
	}
}

func TestGenerateQuoteTestReturnsCopies(t *testing.T) {
	res, dir := newTestResources(t)
	writeUserResource(t, dir, "quotes", "one", `[{"text": "the only quote"}]`)

	// The app reflows the text of the tests it is given.
	gen := mustGen(generateQuoteTest(newRand(), res, "one"))
	gen()[0].Text = "reflowed"

	if got := gen()[0].Text; got != "the only quote" {
		t.Errorf("second draw = %q, want the quote unchanged", got)
	}
}

// newRand returns a source of randomness of its own to the calling test,
// making the same draws on every run so that tests relying on chance can't
// fail randomly.
func newRand() *rand.Rand {
	return rand.New(rand.NewSource(1))
}

// The same source of randomness makes the same tests.
func TestGeneratorsAreReproducible(t *testing.T) {
	res, _ := newTestResources(t)
	st := &memStore{mistakes: []mistake{{"alpha", "x"}, {"beta", "x"}, {"gamma", "x"}}, keys: weakTH()}

	for _, args := range [][]string{
		{"-punct", "-numbers"},
		{"-quotes", "en"},
		{"-mistakes", "-punct"},
		{"-adaptive", "-numbers"},
	} {
		tests := func() [][]segment {
			gen, _, err := newTestSource(parseFlags(args), textSources{st, res, newRand(), strings.NewReader(""), true})
			if err != nil {
				t.Fatal(err)
			}
			return [][]segment{nextTest(t, gen), nextTest(t, gen), nextTest(t, gen)}
		}

		if first, second := tests(), tests(); !reflect.DeepEqual(first, second) {
			t.Errorf("tt %s made\n%q\nthen\n%q", strings.Join(args, " "), first, second)
		}
	}
}
