package textgen

import (
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/alexmarchi28/tt/internal/keystats"
	"github.com/alexmarchi28/tt/internal/typing"
)

// nextTest returns the next test of src, failing on errors.
func nextTest(t *testing.T, src Source) []Segment {
	t.Helper()

	test, err := src.Next()
	if err != nil && err != io.EOF {
		t.Fatal(err)
	}

	return test
}

// nextFunc returns a function returning the successive tests of src, nil
// once there are no more.
func nextFunc(t *testing.T, src Source) func() []Segment {
	return func() []Segment { return nextTest(t, src) }
}

func mustSource[S Source](src S, err error) S {
	if err != nil {
		panic(err)
	}

	return src
}

func TestFuncSource(t *testing.T) {
	t.Parallel()

	var calls int
	src := Func(testSequence(&calls, "a", "b"))

	for _, want := range []string{"a", "b"} {
		if got, err := src.Next(); err != nil || !reflect.DeepEqual(got, []Segment{{want, ""}}) {
			t.Errorf("next() = %q, %v, want %q", got, err, want)
		}
	}
	if got, err := src.Next(); got != nil || err != io.EOF {
		t.Errorf("next() at the end = %q, %v, want io.EOF", got, err)
	}
}

// A paragraph reached which can't be saved ends the session rather than
// crashing tt.
func TestFileSourceReportsSaveErrors(t *testing.T) {
	t.Parallel()

	path := writeTextFile(t, "p0\n\np1\n")
	st := &memStore{}
	src := mustSource(NewFile(path, -1, st))

	st.err = errors.New("read-only")
	if _, err := src.Next(); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Errorf("next() = %v, want the error", err)
	}
}

// countingKeyStats counts the reads of the key statistics.
type countingKeyStats struct {
	keyStatsOf
	reads int
}

func (c *countingKeyStats) KeyStats() keystats.Stats {
	c.reads++
	return c.keyStatsOf.KeyStats()
}

func TestAdaptiveSource(t *testing.T) {
	t.Parallel()

	res, dir := newTestResources(t)
	writeUserResource(t, dir, "words", "list", "the her err rare")
	stats := &countingKeyStats{keyStatsOf: keyStatsOf(weakTH())}

	src := mustSource(NewAdaptive(newRand(), res, "list", WordOptions{N: 5, G: 4}, 40, stats))

	if got := src.Describe(); got != "Practicing th" {
		t.Errorf("describe() = %q", got)
	}

	stats.reads = 0
	if test := nextTest(t, src); len(test) != 4 {
		t.Errorf("next() = %q, want 4 groups of words", test)
	}
	if stats.reads != 1 {
		t.Errorf("the key statistics were read %d times for a test, want once", stats.reads)
	}
}

// The exported constructors make sources of the tests of their generators,
// or report why they can't.
func TestConstructors(t *testing.T) {
	t.Parallel()

	res, dir := newTestResources(t)
	writeUserResource(t, dir, "words", "greek", "alpha beta")
	writeUserResource(t, dir, "quotes", "mine", `[{"text": "Be brief.", "attribution": "Me"}]`)
	o := WordOptions{N: 2, G: 1}
	mistyped := &memStore{mistakes: []typing.Mistake{{Word: "teh", Typed: "the"}}}

	tests := []struct {
		name     string
		make     func() (Source, error)
		failWith func() (Source, error)
		check    func(t *testing.T, segments []Segment)
	}{
		{"words",
			func() (Source, error) { return Words(newRand(), res, "greek", o) },
			func() (Source, error) { return Words(newRand(), res, "nope", o) },
			func(t *testing.T, segments []Segment) {
				if len(segments) != 1 {
					t.Fatalf("got %q, want a segment", segments)
				}
				assertWords(t, segments[0].Text, 2)
			}},
		{"quotes",
			func() (Source, error) { return Quotes(newRand(), res, "mine") },
			func() (Source, error) { return Quotes(newRand(), res, "nope") },
			equals(Segment{Text: "Be brief.", Attribution: "Me"})},
		{"mistakes",
			func() (Source, error) { return Mistakes(newRand(), o, mistyped) },
			func() (Source, error) { return Mistakes(newRand(), o, &memStore{}) },
			equals(Segment{Text: "teh teh"})},
		{"text",
			func() (Source, error) { return Text([]byte("one\n\ntwo"), false, false) },
			func() (Source, error) { return Text([]byte(" \n"), false, false) },
			equals(Segment{Text: "one"}, Segment{Text: "two"})},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.check(t, nextTest(t, mustSource(tc.make())))

			if _, err := tc.failWith(); err == nil {
				t.Error("no error without anything to type, want one")
			}
		})
	}
}

// equals checks that the segments are the given ones.
func equals(want ...Segment) func(t *testing.T, segments []Segment) {
	return func(t *testing.T, segments []Segment) {
		t.Helper()

		if !reflect.DeepEqual(segments, want) {
			t.Errorf("got %q, want %q", segments, want)
		}
	}
}
