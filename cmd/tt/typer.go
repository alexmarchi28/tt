package main

import (
	"fmt"
	"runtime"
	"time"

	"github.com/alexmarchi28/tt/internal/typing"
	"github.com/gdamore/tcell"
)

// outcome is how a test ended.
type outcome int

const (
	outcomeComplete outcome = iota // The text was typed, or the time is up.
	outcomeQuit
	outcomeRestart
	outcomePrevious
	outcomeNext
)

type segment struct {
	Text        string `json:"text"`
	Attribution string `json:"attribution"`
}

// paceTick is the longest interval between screen updates with a pacer.
const paceTick = 50 * time.Millisecond

// typer runs the event loop of tests: it hands the keys pressed to the
// session typed and draws its frames with a view.
type typer struct {
	scr        tcell.Screen
	view       *view
	keys       keymap
	session    sessionDeps // Given to the session of each segment.
	tick       time.Duration
	cursor     cursorShaper
	cycleTheme func(direction int) (string, error) // Switches to another theme and returns its name.

	statusMessage      string
	statusMessageUntil time.Time
	showHelp           bool
}

// typerOption configures a typer, see newTyper.
type typerOption func(*typer)

func withRules(rules typingRules) typerOption {
	return func(t *typer) { t.session.rules = rules }
}

// withPace shows a pacer going through the text at the given speed in WPM.
func withPace(wpm int) typerOption {
	return func(t *typer) { t.session.pace = wpm }
}

// withWrap lays out the text with wrap rather than showing it as is.
func withWrap(wrap func(text string, screenWidth int) string) typerOption {
	return func(t *typer) { t.session.wrap = wrap }
}

// withThemeCycler lets the keys of the keymap switch to the next or previous
// theme with cycle.
func withThemeCycler(cycle func(direction int) (string, error)) typerOption {
	return func(t *typer) { t.cycleTheme = cycle }
}

func withClock(now func() time.Time) typerOption {
	return func(t *typer) { t.session.now = now }
}

// withTick sets the interval between screen updates.
func withTick(tick time.Duration) typerOption {
	return func(t *typer) { t.tick = tick }
}

func withCursor(c cursorShaper) typerOption {
	return func(t *typer) { t.cursor = c }
}

func withKeymap(km keymap) typerOption {
	return func(t *typer) { t.keys = km }
}

// newTyper returns a typer drawing on scr with v. By default it uses the
// keys of the running system and the system clock, and leaves the cursor
// shape as it is.
func newTyper(scr tcell.Screen, v *view, opts ...typerOption) *typer {
	t := &typer{
		scr:     scr,
		view:    v,
		keys:    newKeymap(runtime.GOOS),
		session: sessionDeps{now: time.Now},
		tick:    500 * time.Millisecond,
		cursor:  keptCursor{},
	}

	for _, opt := range opts {
		opt(t)
	}

	return t
}

// showStatusMessage shows a message above the text for the given duration,
// or until it is replaced if the duration is not positive.
func (t *typer) showStatusMessage(message string, duration time.Duration) {
	t.statusMessage = message
	if duration <= 0 {
		t.statusMessageUntil = time.Time{}
		return
	}

	t.statusMessageUntil = t.session.now().Add(duration)
}

func (t *typer) statusMessageActive() string {
	if t.statusMessage == "" {
		return ""
	}

	if !t.statusMessageUntil.IsZero() && t.session.now().After(t.statusMessageUntil) {
		t.statusMessage = ""
		t.statusMessageUntil = time.Time{}
		return ""
	}

	return t.statusMessage
}

// testResult measures the typing of a test.
type testResult struct {
	duration     time.Duration
	correctChars int // Correctly typed characters, used for the speed.
	correctKeys  int // Correct keystrokes, used for the accuracy.
	wrongKeys    int // Wrong keystrokes, used for the accuracy.
	mistakes     []typing.Mistake
	keystrokes   []typing.Keystroke // Segments are separated by a break.
}

func (r testResult) cpm() int {
	if r.duration <= 0 {
		return 0
	}

	return int(float64(r.correctChars) / (float64(r.duration) / 60e9))
}

func (r testResult) wpm() int {
	return r.cpm() / 5
}

func (r testResult) accuracy() float64 {
	return calcAccuracy(r.correctKeys, r.wrongKeys)
}

// Start runs a test made of the given segments and returns how it ended
// along with the typing of the segments typed. A timeout of -1 disables the
// time limit.
func (t *typer) Start(text []segment, timeout time.Duration) (o outcome, res testResult) {
	timeLeft := timeout

	for i, s := range text {
		var r testResult
		o, r = t.start(s.Text, timeLeft, i > 0, s.Attribution)

		res.duration += r.duration
		res.correctChars += r.correctChars
		res.correctKeys += r.correctKeys
		res.wrongKeys += r.wrongKeys
		res.mistakes = append(res.mistakes, r.mistakes...)
		if i > 0 {
			res.keystrokes = append(res.keystrokes, typing.Keystroke{})
		}
		res.keystrokes = append(res.keystrokes, r.keystrokes...)

		if timeout != -1 {
			timeLeft -= r.duration
			if timeLeft <= 0 {
				return
			}
		}

		if o != outcomeComplete {
			return
		}
	}

	return
}

// start runs the typing of a single segment. The timer starts with the first
// key press, unless startImmediately is set.
func (t *typer) start(text string, timeLimit time.Duration, startImmediately bool, attribution string) (outcome, testResult) {
	sw, sh := t.scr.Size()
	s := newSession(text, attribution, timeLimit, sw, sh, t.session)
	if startImmediately {
		s.startTimer()
	}

	t.cursor.typing()
	defer t.cursor.restore()

	stopTicker := t.startTicker()
	defer stopTicker()

	t.view.clear()
	for {
		t.draw(s)

		ev := t.scr.PollEvent()
		if s.timeUp() { //Whatever happened after the time limit comes too late.
			return outcomeComplete, s.result()
		}

		switch ev := ev.(type) {
		case *tcell.EventResize:
			s.resize(t.scr.Size())
			t.view.clear()
		case *tcell.EventKey:
			if o, ended := t.handleKey(s, ev); ended {
				if o != outcomeComplete {
					return o, testResult{}
				}

				return o, s.result()
			}
		default: //The ticks of startTicker only update the screen.
		}
	}
}

// draw draws the frame of s with the status message and help of the typer.
func (t *typer) draw(s *session) {
	f := s.frame()
	f.status = t.statusMessageActive()
	f.help = t.keys.help()
	f.showHelp = t.showHelp

	t.view.draw(f)
}

// handleKey handles a key pressed while typing s and returns the outcome of
// the test if it ended.
func (t *typer) handleKey(s *session, ev *tcell.EventKey) (outcome, bool) {
	switch t.keys.action(ev) {
	case actionQuit:
		return outcomeQuit, true
	case actionRestart:
		return outcomeRestart, true
	case actionNext:
		return outcomeNext, true
	case actionPrevious:
		return outcomePrevious, true
	case actionRedraw:
		t.view.sync()
	case actionNextTheme:
		t.switchTheme(1)
	case actionPreviousTheme:
		t.switchTheme(-1)
	case actionDeleteChar:
		s.correct(false)
	case actionDeleteWord:
		s.correct(true)
	case actionToggleHelp:
		if !s.state.Expects(ev.Rune()) {
			t.showHelp = !t.showHelp
		} else if s.typeRune(ev.Rune()) {
			return outcomeComplete, true
		}
	case actionNone:
		switch ev.Key() {
		case tcell.KeyTab:
			return outcomeComplete, s.typeRune('\t')
		case tcell.KeyRune:
			return outcomeComplete, s.typeRune(ev.Rune())
		}
	}

	return outcomeComplete, false
}

// startTicker injects nil events into the event loop at regular intervals to
// update the screen, until the returned function is called.
func (t *typer) startTicker() (stop func()) {
	done := make(chan struct{})
	tick := t.refreshInterval()

	go func() {
		for {
			select {
			case <-done:
				return
			default:
			}

			time.Sleep(tick)
			t.scr.PostEventWait(nil)
		}
	}()

	return func() { close(done) }
}

// refreshInterval returns the interval between screen updates. The pacer
// moves smoothly only if the screen is updated often.
func (t *typer) refreshInterval() time.Duration {
	if t.session.pace > 0 && t.tick > paceTick {
		return paceTick
	}

	return t.tick
}

// switchTheme switches to the next (direction > 0) or previous theme and
// briefly shows its name.
func (t *typer) switchTheme(direction int) {
	if t.cycleTheme == nil {
		return
	}

	themeName, err := t.cycleTheme(direction)
	if err != nil {
		t.showStatusMessage(err.Error(), 2*time.Second)
	} else if themeName != "" {
		t.showStatusMessage(fmt.Sprintf("Theme: %s", themeName), 2*time.Second)
	}
}

// calcAccuracy returns the percentage of correct keystrokes, or 0 when
// nothing has been typed yet.
func calcAccuracy(correct, errors int) float64 {
	if correct+errors == 0 {
		return 0
	}

	return float64(correct) / float64(errors+correct) * 100
}
