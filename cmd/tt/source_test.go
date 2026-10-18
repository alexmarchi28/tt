package main

import (
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/gdamore/tcell"
)

// nextTest returns the next test of src, failing on errors.
func nextTest(t *testing.T, src testSource) []segment {
	t.Helper()

	test, err := src.next()
	if err != nil && err != io.EOF {
		t.Fatal(err)
	}

	return test
}

// nextFunc returns a function returning the successive tests of src, nil
// once there are no more.
func nextFunc(t *testing.T, src testSource) func() []segment {
	return func() []segment { return nextTest(t, src) }
}

func mustSource[S testSource](src S, err error) S {
	if err != nil {
		panic(err)
	}

	return src
}

func TestFuncSource(t *testing.T) {
	var calls int
	src := funcSource(testSequence(&calls, "a", "b"))

	for _, want := range []string{"a", "b"} {
		if got, err := src.next(); err != nil || !reflect.DeepEqual(got, []segment{{want, ""}}) {
			t.Errorf("next() = %q, %v, want %q", got, err, want)
		}
	}
	if got, err := src.next(); got != nil || err != io.EOF {
		t.Errorf("next() at the end = %q, %v, want io.EOF", got, err)
	}
}

// A paragraph reached which can't be saved ends the session rather than
// crashing tt.
func TestFileSourceReportsSaveErrors(t *testing.T) {
	path := writeTextFile(t, "p0\n\np1\n")
	st := &memStore{}
	src := mustSource(newFileSource(path, -1, st))

	st.err = errors.New("read-only")
	if _, err := src.next(); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Errorf("next() = %v, want the error", err)
	}
}

// countingKeyStats counts the reads of the key statistics.
type countingKeyStats struct {
	keyStatsOf
	reads int
}

func (c *countingKeyStats) KeyStats() keyStats {
	c.reads++
	return c.keyStatsOf.KeyStats()
}

func TestAdaptiveSource(t *testing.T) {
	res, dir := newTestResources(t)
	writeUserResource(t, dir, "words", "list", "the her err rare")
	stats := &countingKeyStats{keyStatsOf: keyStatsOf(weakTH())}

	src := mustSource(newAdaptiveSource(newRand(), res, "list", wordOptions{n: 5, g: 4}, 40, stats))

	if got := src.describe(); got != "Practicing th" {
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

// errSource fails to produce a test.
type errSource struct{}

func (errSource) next() ([]segment, error) { return nil, errors.New("no test") }

func TestAppStopsOnSourceErrors(t *testing.T) {
	a, _ := newTestApp(t, resolveFlags(nil))

	if rc, err := a.run(errSource{}); rc != 1 || err == nil || err.Error() != "no test" {
		t.Errorf("run() = %d, %v, want the error of the source", rc, err)
	}
}

// describedSource is a test source describing what it practices.
type describedSource struct {
	testSource
	description string
}

func (s describedSource) describe() string { return s.description }

func TestAppShowsWhatSourcesPractice(t *testing.T) {
	a, sim := newTestApp(t, resolveFlags([]string{"-noreport"}))

	var calls int
	done := startSource(a, describedSource{funcSource(testSequence(&calls, "ab")), "Practicing ab"})
	waitForScreenText(t, sim, "Practicing ab", true)

	sendKeys(t, sim, tcell.KeyCtrlC)
	waitExitCode(t, done)
}
