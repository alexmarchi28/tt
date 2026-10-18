package main

import (
	"io"
	"reflect"
	"testing"
)

// The tests are made when first needed and kept, so that going back shows
// the same tests again.
func TestTestHistory(t *testing.T) {
	t.Parallel()

	var calls int
	h := newTestHistory(funcSource(testSequence(&calls, "ab", "cd")))

	current := func(want string) {
		t.Helper()
		got, err := h.current()
		if err != nil || !reflect.DeepEqual(got, []segment{{want, ""}}) {
			t.Fatalf("current() = %q, %v, want %q", got, err, want)
		}
	}

	h.previous() // Nothing before the first test.
	current("ab")
	current("ab")
	h.next()
	current("cd")
	h.previous()
	current("ab")
	h.next()
	current("cd")

	if calls != 2 {
		t.Errorf("tests made %d times, want 2", calls)
	}

	h.next()
	if _, err := h.current(); err != io.EOF {
		t.Errorf("current() after the last test: error %v, want io.EOF", err)
	}
}

func TestTestHistoryFailure(t *testing.T) {
	t.Parallel()

	h := newTestHistory(errSource{})

	if _, err := h.current(); err == nil || err.Error() != "no test" {
		t.Errorf("current() error %v, want the error of the source", err)
	}
}
