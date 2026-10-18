package main

import (
	"testing"
	"time"
)

// A session is typed and laid out without a screen.
func TestSessionFrame(t *testing.T) {
	t.Parallel()

	clock := newFakeClock()
	s := newSession("abcdefghij", "- me", 10*time.Second, 20, 10, sessionDeps{now: clock.now, pace: 60})

	f := s.frame()
	if f.x != 5 || f.y != 4 || f.nc != 10 || f.nr != 1 {
		t.Errorf("text at (%d, %d) of size %dx%d, want it centered at (5, 4)", f.x, f.y, f.nc, f.nr)
	}
	if f.pace != -1 || f.stats != nil || f.timed || f.attribution != "- me" {
		t.Errorf("frame before the first key = %+v, want no pacer, statistics nor timer", f)
	}

	s.typeRune('a')
	clock.advance(time.Second)

	// 60 wpm is 5 characters per second.
	f = s.frame()
	if f.idx != 1 || f.pace != 5 || !f.timed || f.timeLeft != 9*time.Second {
		t.Errorf("frame after a second = idx %d, pace %d, timed %t, time left %v", f.idx, f.pace, f.timed, f.timeLeft)
	}
	if f.stats == nil || f.stats.correctChars != 1 || f.stats.duration != time.Second {
		t.Errorf("live statistics = %+v, want a character in a second", f.stats)
	}
}

func TestSessionResize(t *testing.T) {
	t.Parallel()

	s := newSession("alpha beta", "", -1, 40, 10, sessionDeps{now: newFakeClock().now, wrap: wrapAtScreenWidth})
	s.typeRune('a')

	s.resize(6, 10)

	if f := s.frame(); string(f.text) != "alpha \nbeta" || f.idx != 1 || f.x != 0 || f.nr != 2 {
		t.Errorf("frame after resizing = %q at %d, x %d, %d rows", string(f.text), f.idx, f.x, f.nr)
	}
}

func TestSessionRules(t *testing.T) {
	t.Parallel()

	s := newSession("ab cd", "", -1, 40, 10, sessionDeps{now: newFakeClock().now, rules: typingRules{noSkip: true, noBackspace: true}})

	s.typeRune('a')
	s.typeRune(' ')
	s.correct(false)

	if s.state.Cursor() != 2 || s.state.WrongKeys() != 1 {
		t.Errorf("cursor at %d with %d wrong keys, want the space typed and kept", s.state.Cursor(), s.state.WrongKeys())
	}
}
