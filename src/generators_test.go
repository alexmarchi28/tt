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

	segments := generateWordTest("1000en", 50, 1)()
	if len(segments) != 1 {
		t.Fatalf("got %d segments, want 1", len(segments))
	}
	if n := len(strings.Fields(segments[0].Text)); n < 45 || n > 50 {
		t.Errorf("got %d words, want about 50", n)
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
