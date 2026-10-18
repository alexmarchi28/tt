package main

import (
	"time"

	"github.com/alexmarchi28/tt/internal/typing"
)

// typingRules say how the keys typed are handled.
type typingRules struct {
	noSkip      bool // Space is typed like any character, rather than skipping the current word.
	noBackspace bool // Typed characters can't be deleted.
}

// sessionDeps are what a session is typed with.
type sessionDeps struct {
	rules typingRules
	now   func() time.Time
	wrap  func(text string, screenWidth int) string // Lays out the text on the screen, nil to show it as is.
	pace  int                                       // Speed in WPM of a pacer going through the text, 0 for none.
}

// session is the typing of a single segment of a test, laid out on a screen
// of a given size.
type session struct {
	sessionDeps
	state       *typing.State
	attribution string
	timeLimit   time.Duration // -1 for no time limit.
	startTime   time.Time     // Zero until the timer starts.

	sw, sh int // Size of the screen.
	x, y   int // Position of the text.
	nc, nr int // Size of the text.
}

func newSession(text, attribution string, timeLimit time.Duration, sw, sh int, deps sessionDeps) *session {
	s := &session{
		sessionDeps: deps,
		attribution: attribution,
		timeLimit:   timeLimit,
		sw:          sw,
		sh:          sh,
	}

	s.state = typing.NewState(s.layout(text))
	s.center()

	return s
}

// layout lays out text on the screen.
func (s *session) layout(text string) string {
	if s.wrap == nil {
		return text
	}

	return s.wrap(text, s.sw)
}

// center places the text at the center of the screen.
func (s *session) center() {
	s.nc, s.nr = calcStringDimensions(string(s.state.Text()))
	s.x = (s.sw - s.nc) / 2
	s.y = (s.sh - s.nr) / 2
}

// resize lays out the text again for a screen of the given size, keeping
// what was typed.
func (s *session) resize(sw, sh int) {
	s.sw, s.sh = sw, sh
	s.state.Relayout(s.layout(string(s.state.Text())))
	s.center()
}

func (s *session) startTimer() {
	if s.startTime.IsZero() {
		s.startTime = s.now()
	}
}

func (s *session) started() bool {
	return !s.startTime.IsZero()
}

// elapsed returns the time since the timer started, 0 before it does.
func (s *session) elapsed() time.Duration {
	if !s.started() {
		return 0
	}

	return s.now().Sub(s.startTime)
}

// timeUp reports whether the time limit is reached.
func (s *session) timeUp() bool {
	return s.timeLimit != -1 && s.timeLimit <= s.elapsed()
}

// duration returns the time spent typing, which ends at the time limit.
func (s *session) duration() time.Duration {
	if s.timeUp() {
		return s.timeLimit
	}

	return s.elapsed()
}

func (s *session) result() testResult {
	return testResult{
		duration:     s.duration(),
		correctChars: s.state.CorrectChars(),
		correctKeys:  s.state.CorrectKeys(),
		wrongKeys:    s.state.WrongKeys(),
		mistakes:     s.state.Mistakes(),
		keystrokes:   s.state.Keystrokes(),
	}
}

// typeRune types a character and reports whether the text is complete.
func (s *session) typeRune(r rune) bool {
	if !s.state.Done() {
		s.startTimer()
		s.state.Type(r, !s.rules.noSkip, s.elapsed())
	}

	return s.state.Done()
}

// correct deletes the previous character, or word, unless backspace is
// disabled.
func (s *session) correct(word bool) {
	if s.rules.noBackspace {
		return
	}

	s.startTimer()
	if word {
		s.state.DeleteWord()
	} else {
		s.state.Backspace()
	}
}

// frame returns what the screen shows of the session.
func (s *session) frame() frame {
	f := frame{
		text:        s.state.Text(),
		typed:       s.state.Typed(),
		idx:         s.state.Cursor(),
		x:           s.x,
		y:           s.y,
		nc:          s.nc,
		nr:          s.nr,
		pace:        -1,
		attribution: s.attribution,
	}

	if s.started() {
		if s.pace > 0 {
			f.pace = typing.PaceIndex(s.state.Text(), s.elapsed(), s.pace)
		}
		if s.timeLimit != -1 {
			f.timed, f.timeLeft = true, s.timeLimit-s.elapsed()
		}

		//Only what the live statistics need, see view.drawLiveStats.
		f.stats = &testResult{
			duration:     s.duration(),
			correctChars: s.state.CorrectChars(),
			correctKeys:  s.state.CorrectKeys(),
			wrongKeys:    s.state.WrongKeys(),
		}
	}

	return f
}
