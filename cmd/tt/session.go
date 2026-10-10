package main

import (
	"runtime"
	"strconv"
	"time"

	"github.com/gdamore/tcell"
)

// session is the typing of a single segment of a test.
type session struct {
	t           *typer
	state       *typingState
	attribution string
	timeLimit   time.Duration // -1 for no time limit.
	startTime   time.Time     // Zero until the timer starts.

	sw, sh int // Size of the screen.
	x, y   int // Position of the text.
	nc, nr int // Size of the text.
}

func (t *typer) newSession(text string, timeLimit time.Duration, attribution string) *session {
	s := &session{
		t:           t,
		state:       newTypingState(text),
		attribution: attribution,
		timeLimit:   timeLimit,
	}

	s.sw, s.sh = t.Scr.Size()
	s.nc, s.nr = calcStringDimensions(text)
	s.x = (s.sw - s.nc) / 2
	s.y = (s.sh - s.nr) / 2

	return s
}

func (s *session) startTimer() {
	if s.startTime.IsZero() {
		s.startTime = s.t.now()
	}
}

// elapsed returns the time since the timer started, 0 before it does.
func (s *session) elapsed() time.Duration {
	if s.startTime.IsZero() {
		return 0
	}

	return s.t.now().Sub(s.startTime)
}

// timeUp reports whether the time limit is reached.
func (s *session) timeUp() bool {
	return s.timeLimit != -1 && s.timeLimit <= s.elapsed()
}

func (s *session) result() testResult {
	return testResult{
		rc:           TyperComplete,
		duration:     s.elapsed(),
		correctChars: s.state.correctChars(),
		correctKeys:  s.state.correctKeys,
		wrongKeys:    s.state.wrongKeys,
		mistakes:     s.state.mistakes(),
		keystrokes:   s.state.keystrokes,
	}
}

// typeRune types a character and reports whether the text is complete.
func (s *session) typeRune(r rune) bool {
	if !s.state.done() {
		s.startTimer()
		s.state.typeRune(r, s.t.SkipWord, s.elapsed())
	}

	return s.state.done()
}

// correct deletes the previous character, or word, unless backspace is
// disabled.
func (s *session) correct(word bool) {
	if s.t.DisableBackspace {
		return
	}

	s.startTimer()
	if word {
		s.state.deleteWord()
	} else {
		s.state.backspace()
	}
}

// handleKey handles a key press and returns the outcome of the test if it
// ended.
func (s *session) handleKey(ev *tcell.EventKey) (testResult, bool) {
	t := s.t

	if runtime.GOOS != "windows" && ev.Key() == tcell.KeyBackspace { //Control+backspace on unix terms
		s.correct(true)
		return testResult{}, false
	}

	switch ev.Key() {
	case tcell.KeyCtrlC:
		return testResult{rc: TyperSigInt}, true
	case tcell.KeyEscape:
		return testResult{rc: TyperEscape}, true
	case tcell.KeyRight:
		return testResult{rc: TyperNext}, true
	case tcell.KeyLeft:
		return testResult{rc: TyperPrevious}, true
	case tcell.KeyCtrlL:
		t.Scr.Sync()
	case tcell.KeyCtrlN:
		t.cycleTheme(1)
	case tcell.KeyCtrlP:
		t.cycleTheme(-1)
	case tcell.KeyCtrlW:
		s.correct(true)
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		s.correct(ev.Modifiers() == tcell.ModAlt || ev.Modifiers() == tcell.ModCtrl)
	case tcell.KeyTab:
		if s.typeRune('\t') {
			return s.result(), true
		}
	case tcell.KeyRune:
		if ev.Rune() == '?' && !s.state.expects('?') {
			t.showKeyboardHelp = !t.showKeyboardHelp
		} else if s.typeRune(ev.Rune()) {
			return s.result(), true
		}
	}

	return testResult{}, false
}

func (s *session) redraw() {
	t := s.t

	pace := -1
	if t.Pace > 0 && !s.startTime.IsZero() {
		pace = paceIndex(s.state.text, s.elapsed(), t.Pace)
	}

	t.clearKeyboardHelpArea(s.sw, s.sh)
	t.drawText(s.state, s.x, s.y, pace)

	aw, ah := calcStringDimensions(s.attribution)
	drawString(t.Scr, s.x+s.nc-aw, s.y+s.nr+1, s.attribution, t.defaultStyle)

	if s.timeLimit != -1 && !s.startTime.IsZero() {
		remaining := s.timeLimit - s.elapsed()
		drawString(t.Scr, s.x+s.nc/2, s.y+s.nr+ah+1, "      ", t.defaultStyle)
		drawString(t.Scr, s.x+s.nc/2, s.y+s.nr+ah+1, strconv.Itoa(int(remaining/1e9)+1), t.defaultStyle)
	}

	if (t.ShowWpm || t.ShowAccuracy) && !s.startTime.IsZero() {
		t.drawLiveStats(s.x+s.nc/2-4, s.y-2, s.result())
	}

	t.drawStatusMessage(s.sw, s.sh)

	if t.showKeyboardHelp {
		t.drawKeyboardHelp(s.sw, s.sh)
	}

	//Potentially inefficient, but seems to be good enough
	t.Scr.Show()
}
