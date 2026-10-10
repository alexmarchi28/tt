package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestGenerateTestFromDataRaw(t *testing.T) {
	gen := generateTestFromData([]byte("one\n\ntwo  three\n"), true, false)

	for i := 0; i < 2; i++ {
		want := []segment{{"one\n\ntwo  three\n", ""}}
		if got := gen(); !reflect.DeepEqual(got, want) {
			t.Fatalf("call %d: got %q, want %q", i, got, want)
		}
	}
}

func TestGenerateTestFromDataMulti(t *testing.T) {
	gen := generateTestFromData([]byte("one\n\ntwo\nthree\n"), false, true)

	for _, want := range [][]segment{{{"one", ""}}, {{"two\nthree", ""}}, nil, nil} {
		if got := gen(); !reflect.DeepEqual(got, want) {
			t.Fatalf("got %q, want %q", got, want)
		}
	}
}

func TestGenerateTestFromDataParagraphs(t *testing.T) {
	gen := generateTestFromData([]byte("one\n\ntwo\nthree\n"), false, false)

	want := []segment{{"one", ""}, {"two\nthree", ""}}
	for i := 0; i < 2; i++ {
		if got := gen(); !reflect.DeepEqual(got, want) {
			t.Fatalf("call %d: got %q, want %q", i, got, want)
		}
	}
}

func TestGenerateWordTest(t *testing.T) {
	dir := useTempStorage(t)
	writeUserResource(t, dir, "words", "greek", "alpha beta\tgamma\ndelta")

	gen := generateWordTest("greek", 3, 2)
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
	useTempStorage(t)

	gen := generateWordTest("1000en", 50, 1)
	for i := 0; i < 100; i++ {
		segments := gen()
		if len(segments) != 1 {
			t.Fatalf("got %d segments, want 1", len(segments))
		}
		assertWords(t, segments[0].Text, 50)
	}
}

func TestGenerateWordTestIgnoresBlankEntries(t *testing.T) {
	dir := useTempStorage(t)
	writeUserResource(t, dir, "words", "blanks", "\n  alpha beta\n\ngamma\t\n")

	gen := generateWordTest("blanks", 10, 1)
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
	dir := useTempStorage(t)
	writeUserResource(t, dir, "quotes", "mine", `[
		{"text": "first quote", "attribution": "someone"},
		{"text": "second quote", "attribution": "someone else"}
	]`)

	gen := generateQuoteTest("mine")
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
	useTempStorage(t)

	for i := 0; i < 20; i++ {
		if got := generateQuoteTest("en")(); len(got) != 1 || got[0].Text == "" {
			t.Fatalf("unexpected builtin quote %q", got)
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
	useTempStorage(t)
	path := writeTextFile(t, "p0\n\np1\nstill p1\n\np2\n")

	gen := generateTestFromFile(path, -1)
	assertNextParagraph(t, gen, "p0")
	assertNextParagraph(t, gen, "p1\nstill p1")
	assertNextParagraph(t, gen, "p2")
	assertNextParagraph(t, gen, "")
}

func TestGenerateTestFromFileRemembersPosition(t *testing.T) {
	useTempStorage(t)
	path := writeTextFile(t, "p0\n\np1\n\np2\n")

	gen := generateTestFromFile(path, -1)
	assertNextParagraph(t, gen, "p0")
	assertNextParagraph(t, gen, "p1")

	// A new run resumes from the paragraph that was last shown.
	gen = generateTestFromFile(path, -1)
	assertNextParagraph(t, gen, "p1")
	assertNextParagraph(t, gen, "p2")
	assertNextParagraph(t, gen, "")

	gen = generateTestFromFile(path, -1)
	assertNextParagraph(t, gen, "")

	// -start overrides (and resets) the stored position.
	gen = generateTestFromFile(path, 0)
	assertNextParagraph(t, gen, "p0")

	gen = generateTestFromFile(path, 2)
	assertNextParagraph(t, gen, "p2")
}

func TestGenerateTestFromFileTracksFilesSeparately(t *testing.T) {
	useTempStorage(t)
	a := writeTextFile(t, "a0\n\na1\n")
	b := writeTextFile(t, "b0\n\nb1\n")

	assertNextParagraph(t, generateTestFromFile(a, 1), "a1")
	assertNextParagraph(t, generateTestFromFile(b, -1), "b0")
	assertNextParagraph(t, generateTestFromFile(a, -1), "a1")
}

func TestNewTestGenerator(t *testing.T) {
	useTempStorage(t)
	book := writeTextFile(t, "first paragraph\n\nsecond paragraph\n")
	saveTestMistakes(t, "ab")

	tests := []struct {
		name            string
		args            []string
		stdin           string
		stdinIsTerminal bool
		check           func(t *testing.T, segments []segment)
		kind            testKind
	}{
		{"default", nil, "", true, wordsPerSegment(50), testKind{"words", "1000en"}},
		{"word list", []string{"-words", "200en", "-n", "3", "-g", "2"}, "", true, wordsPerSegment(3, 3), testKind{"words", "200en"}},
		{"quotes", []string{"-quotes", "en"}, "", true, func(t *testing.T, segments []segment) {
			if len(segments) != 1 || segments[0].Text == "" {
				t.Errorf("got %q, want a quote", segments)
			}
		}, testKind{"quotes", "en"}},
		{"stdin", nil, "piped text\n\nmore", false, exactly("piped text", "more"), testKind{"text", ""}},
		{"word list over stdin", []string{"-words", "200en", "-n", "4"}, "piped text", false, wordsPerSegment(4), testKind{"words", "200en"}},
		{"file", []string{book}, "", true, exactly("first paragraph"), testKind{"file", "book.txt"}},
		{"stdin over file", []string{book}, "piped text", false, exactly("piped text"), testKind{"text", ""}},
		{"mistakes", []string{"-mistakes", "-n", "3"}, "", true, exactly("ab ab ab"), testKind{"mistakes", ""}},
		{"mistakes over stdin", []string{"-mistakes", "-n", "2"}, "piped text", false, exactly("ab ab"), testKind{"mistakes", ""}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gen, kind := newTestGenerator(parseFlags(tc.args), strings.NewReader(tc.stdin), tc.stdinIsTerminal)
			tc.check(t, gen())

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

func TestTestKindString(t *testing.T) {
	tests := map[testKind]string{
		{"words", "1000en"}: "words 1000en",
		{"text", ""}:        "text",
		{}:                  "unknown",
	}

	for kind, want := range tests {
		if got := kind.String(); got != want {
			t.Errorf("%+v.String() = %q, want %q", kind, got, want)
		}
	}

	if got := (persistedResult{Mode: "quotes", Source: "en"}).kind(); got != (testKind{"quotes", "en"}) {
		t.Errorf("kind() = %+v, want quotes en", got)
	}
}

func saveTestMistakes(t *testing.T, words ...string) {
	t.Helper()

	var mistakes []mistake
	for _, w := range words {
		mistakes = append(mistakes, mistake{w, "x"})
	}
	if err := writeValue(mistakeDB, mistakes); err != nil {
		t.Fatal(err)
	}
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
	useTempStorage(t)

	var words []string
	for i := 0; i < 100; i++ {
		words = append(words, "old")
	}
	for i := 0; i < recentMistakes/10; i++ {
		words = append(words, "often", "often", "often", "often", "often", "often", "often", "often", "rarely", "seldom")
	}
	saveTestMistakes(t, words...)

	gen := generateMistakesTest(50, 2)
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
