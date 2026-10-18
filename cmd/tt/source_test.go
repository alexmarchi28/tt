package main

import (
	"errors"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/alexmarchi28/tt/internal/history"
	"github.com/alexmarchi28/tt/internal/keystats"
	"github.com/alexmarchi28/tt/internal/textgen"
	"github.com/alexmarchi28/tt/internal/typing"
	"github.com/gdamore/tcell"
)

func writeTextFile(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "book.txt")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	return path
}

func mistakesOf(words ...string) *memStore {
	var mistakes []typing.Mistake
	for _, w := range words {
		mistakes = append(mistakes, typing.Mistake{Word: w, Typed: "x"})
	}

	return &memStore{mistakes: mistakes}
}

// nextTest returns the next test of src, failing on errors.
func nextTest(t *testing.T, src textgen.Source) []textgen.Segment {
	t.Helper()

	test, err := src.Next()
	if err != nil && err != io.EOF {
		t.Fatal(err)
	}

	return test
}

func newRand() *rand.Rand {
	return rand.New(rand.NewSource(1))
}

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

func endsSentence(w string) bool {
	return strings.ContainsAny(w[len(w)-1:], ".?!")
}

// weakTH returns statistics where "th" is typed badly and "he" and "er" well.
func weakTH() keystats.Stats {
	ks := keystats.New()
	ks.Bigrams["th"] = keystats.Stat{Count: 10, ErrorRate: 0.3, Timed: 10, Latency: 300}
	ks.Bigrams["he"] = keystats.Stat{Count: 10, ErrorRate: 0, Timed: 10, Latency: 100}
	ks.Bigrams["er"] = keystats.Stat{Count: 10, ErrorRate: 0, Timed: 10, Latency: 100}
	return ks
}

func TestNewTestGenerator(t *testing.T) {
	t.Parallel()

	res, _ := newTestResources(t)
	book := writeTextFile(t, "first paragraph\n\nsecond paragraph\n")
	st := mistakesOf("ab")

	tests := []struct {
		name            string
		args            []string
		stdin           string
		stdinIsTerminal bool
		check           func(t *testing.T, segments []textgen.Segment)
		kind            history.Kind
	}{
		{"default", nil, "", true, wordsPerSegment(50), history.Kind{Mode: "words", Source: "1000en", Words: 50}},
		{"word list", []string{"-words", "200en", "-n", "3", "-g", "2"}, "", true, wordsPerSegment(3, 3), history.Kind{Mode: "words", Source: "200en", Words: 6}},
		{"quotes", []string{"-quotes", "en"}, "", true, func(t *testing.T, segments []textgen.Segment) {
			if len(segments) != 1 || segments[0].Text == "" {
				t.Errorf("got %q, want a quote", segments)
			}
		}, history.Kind{Mode: "quotes", Source: "en"}},
		{"stdin", nil, "piped text\n\nmore", false, exactly("piped text", "more"), history.Kind{Mode: "text"}},
		{"word list over stdin", []string{"-words", "200en", "-n", "4"}, "piped text", false, wordsPerSegment(4), history.Kind{Mode: "words", Source: "200en", Words: 4}},
		{"file", []string{book}, "", true, exactly("first paragraph"), history.Kind{Mode: "file", Source: "book.txt"}},
		{"file over stdin", []string{book}, "piped text", false, exactly("first paragraph"), history.Kind{Mode: "file", Source: "book.txt"}},
		{"mistakes", []string{"-mistakes", "-n", "3"}, "", true, exactly("ab ab ab"), history.Kind{Mode: "mistakes", Words: 3}},
		{"punctuation and numbers", []string{"-punct", "-numbers", "-n", "200"}, "", true, func(t *testing.T, segments []textgen.Segment) {
			if len(segments) != 1 || !endsSentence(segments[0].Text) || !strings.ContainsAny(segments[0].Text, "0123456789") {
				t.Errorf("got %q, want sentences with numbers", segments)
			}
		}, history.Kind{Mode: "words", Source: "1000en", Words: 200, Punct: true, Numbers: true}},
		{"mistakes over stdin", []string{"-mistakes", "-n", "2"}, "piped text", false, exactly("ab ab"), history.Kind{Mode: "mistakes", Words: 2}},
		{"adaptive", []string{"-adaptive", "-n", "5"}, "", true, wordsPerSegment(5), history.Kind{Mode: "adaptive", Source: "1000en", Words: 5, Focus: 40}},
		{"adaptive word list", []string{"-adaptive", "-words", "200en", "-n", "5", "-focus", "70"}, "piped text", false, wordsPerSegment(5), history.Kind{Mode: "adaptive", Source: "200en", Words: 5, Focus: 70}},
		{"timed words", []string{"-t", "10", "-n", "5"}, "", true, lastsAtLeast(10*time.Second, 5), history.Kind{Mode: "words", Source: "1000en", Seconds: 10}},
		{"timed mistakes", []string{"-mistakes", "-t", "10", "-n", "5"}, "", true, lastsAtLeast(10*time.Second, 5), history.Kind{Mode: "mistakes", Seconds: 10}},
		{"timed adaptive", []string{"-adaptive", "-t", "10", "-n", "5"}, "", true, lastsAtLeast(10*time.Second, 5), history.Kind{Mode: "adaptive", Source: "1000en", Seconds: 10, Focus: 40}},
		{"timed quotes", []string{"-quotes", "en", "-t", "30"}, "", true, func(t *testing.T, segments []textgen.Segment) {
			if len(segments) != 1 || segments[0].Text == "" {
				t.Errorf("got %q, want a quote", segments)
			}
		}, history.Kind{Mode: "quotes", Source: "en", Seconds: 30}},
		{"word options over stdin", []string{"-punct", "-numbers", "-n", "3"}, "piped text", false, exactly("piped text"), history.Kind{Mode: "text"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, err := resolve(parseFlags(tc.args), tc.stdinIsTerminal)
			if err != nil {
				t.Fatal(err)
			}
			gen, err := newTestSource(s.source, textSources{st, res, newRand(), strings.NewReader(tc.stdin)})
			if err != nil {
				t.Fatal(err)
			}
			tc.check(t, nextTest(t, gen))

			if s.source.kind != tc.kind {
				t.Errorf("kind = %+v, want %+v", s.source.kind, tc.kind)
			}
		})
	}
}

// wordsPerSegment checks that there is a segment of n words for each given n.
func wordsPerSegment(n ...int) func(t *testing.T, segments []textgen.Segment) {
	return func(t *testing.T, segments []textgen.Segment) {
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
func lastsAtLeast(limit time.Duration, n int) func(t *testing.T, segments []textgen.Segment) {
	return func(t *testing.T, segments []textgen.Segment) {
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

func exactly(texts ...string) func(t *testing.T, segments []textgen.Segment) {
	return func(t *testing.T, segments []textgen.Segment) {
		t.Helper()

		var want []textgen.Segment
		for _, text := range texts {
			want = append(want, textgen.Segment{Text: text})
		}
		if !reflect.DeepEqual(segments, want) {
			t.Errorf("got %q, want %q", segments, want)
		}
	}
}

// The same source of randomness makes the same tests.
func TestGeneratorsAreReproducible(t *testing.T) {
	t.Parallel()

	res, _ := newTestResources(t)
	st := &memStore{mistakes: []typing.Mistake{{Word: "alpha", Typed: "x"}, {Word: "beta", Typed: "x"}, {Word: "gamma", Typed: "x"}}, keys: weakTH()}

	for _, args := range [][]string{
		{"-punct", "-numbers"},
		{"-quotes", "en"},
		{"-mistakes", "-punct"},
		{"-adaptive", "-numbers"},
	} {
		tests := func() [][]textgen.Segment {
			gen, err := newTestSource(resolveFlags(args).source, textSources{st, res, newRand(), strings.NewReader("")})
			if err != nil {
				t.Fatal(err)
			}
			return [][]textgen.Segment{nextTest(t, gen), nextTest(t, gen), nextTest(t, gen)}
		}

		if first, second := tests(), tests(); !reflect.DeepEqual(first, second) {
			t.Errorf("tt %s made\n%q\nthen\n%q", strings.Join(args, " "), first, second)
		}
	}
}

// errSource fails to produce a test.
type errSource struct{}

func (errSource) Next() ([]textgen.Segment, error) { return nil, errors.New("no test") }

func TestAppStopsOnSourceErrors(t *testing.T) {
	t.Parallel()

	a, _ := newTestApp(t, resolveFlags(nil))

	if rc, err := a.run(errSource{}); rc != 1 || err == nil || err.Error() != "no test" {
		t.Errorf("run() = %d, %v, want the error of the source", rc, err)
	}
}

// describedSource is a test source describing what it practices.
type describedSource struct {
	textgen.Source
	description string
}

func (s describedSource) Describe() string { return s.description }

func TestAppShowsWhatSourcesPractice(t *testing.T) {
	t.Parallel()

	a, sim := newTestApp(t, resolveFlags([]string{"-noreport"}))

	var calls int
	done := startSource(a, describedSource{textgen.Func(testSequence(&calls, "ab")), "Practicing ab"})
	waitForScreenText(t, sim, "Practicing ab", true)

	sendKeys(t, sim, tcell.KeyCtrlC)
	waitExitCode(t, done)
}

func TestAppShowsAdaptiveFocus(t *testing.T) {
	t.Parallel()

	res, dir := newTestResources(t)
	writeUserResource(t, dir, "words", "list", "the her err rare")
	s := resolveFlags([]string{"-adaptive", "-words", "list", "-n", "5"})
	src, err := newTestSource(s.source, textSources{&memStore{keys: weakTH()}, res, newRand(), strings.NewReader("")})
	if err != nil {
		t.Fatal(err)
	}

	a, sim := newTestApp(t, resolveFlags([]string{"-noreport"}))
	done := startSource(a, src)
	waitForScreenText(t, sim, "Practicing th", true)

	sendKeys(t, sim, tcell.KeyCtrlC)
	waitExitCode(t, done)
}

func TestAdaptiveWordListFromStdin(t *testing.T) {
	t.Parallel()

	res, _ := newTestResourcesWithStdin(t, "alpha")
	s := resolveFlags([]string{"-adaptive", "-words", "-", "-n", "3"})

	src, err := newTestSource(s.source, textSources{&memStore{}, res, newRand(), strings.NewReader("")})
	if err != nil {
		t.Fatal(err)
	}
	if got := nextTest(t, src); !reflect.DeepEqual(got, []textgen.Segment{{Text: "alpha alpha alpha"}}) {
		t.Errorf("next() = %q, want the words read from stdin", got)
	}
}
