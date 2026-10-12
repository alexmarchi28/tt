package main

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/gdamore/tcell"
)

const (
	TyperComplete = iota
	TyperSigInt
	TyperEscape
	TyperPrevious
	TyperNext
	TyperResize
)

type segment struct {
	Text        string `json:"text"`
	Attribution string `json:"attribution"`
}

type mistake struct {
	Word  string `json:"word"`
	Typed string `json:"typed"`
}

// paceTick is the longest interval between screen updates with a pacer.
const paceTick = 50 * time.Millisecond

type typer struct {
	Scr              tcell.Screen
	SkipWord         bool
	ShowWpm          bool
	ShowAccuracy     bool
	DisableBackspace bool
	BlockCursor      bool
	Blind            bool // Show mistakes like correctly typed characters.
	Pace             int  // Speed in WPM of a pacer shown through the text, 0 for none.
	CycleTheme       func(direction int) (string, error)
	tty              io.Writer
	now              func() time.Time // The clock, replaced in tests.
	tick             time.Duration    // Interval between screen updates.

	emboldenTypedText   bool
	highlightCurrent    bool
	highlightNext       bool
	theme               theme
	currentWordStyle    tcell.Style
	nextWordStyle       tcell.Style
	incorrectSpaceStyle tcell.Style
	incorrectStyle      tcell.Style
	correctStyle        tcell.Style
	defaultStyle        tcell.Style
	statusMessage       string
	statusMessageUntil  time.Time
	showKeyboardHelp    bool
}

var keyboardHelpLines = []string{
	"Help",
	"? - show/hide this help",
	"Esc - restart the current test",
	"C-c - quit",
	"C-l - redraw the terminal",
	"Right - next test",
	"Left - previous test",
	"C-n - next theme",
	"C-p - previous theme",
	"Backspace - delete previous character if enabled",
	"C-w / C-backspace / Alt-backspace - delete previous word if enabled",
	"Space - skip current word if enabled",
}

func NewTyper(scr tcell.Screen, emboldenTypedText bool, th theme) *typer {
	var tty io.Writer

	tty, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	//Will fail on windows, but tt is still mostly usable via tcell
	if err != nil {
		tty = io.Discard
	}

	t := &typer{
		Scr:      scr,
		SkipWord: true,
		tty:      tty,
		now:      time.Now,
		tick:     500 * time.Millisecond,

		emboldenTypedText: emboldenTypedText,
		highlightCurrent:  true,
		highlightNext:     true,
	}

	t.SetTheme(th)
	return t
}

func (t *typer) applyStyles() {
	def := tcell.StyleDefault.
		Foreground(t.theme.fgcol).
		Background(t.theme.bgcol)

	correctStyle := def.Foreground(t.theme.hicol)
	if t.emboldenTypedText {
		correctStyle = correctStyle.Bold(true)
	}

	currentWordStyle := def.Foreground(t.theme.hicol2)
	nextWordStyle := def.Foreground(t.theme.hicol3)
	if !t.highlightNext {
		currentWordStyle = nextWordStyle
		nextWordStyle = def
	}
	if !t.highlightCurrent {
		currentWordStyle = def
	}

	t.defaultStyle = def
	t.correctStyle = correctStyle
	t.currentWordStyle = currentWordStyle
	t.nextWordStyle = nextWordStyle
	t.incorrectStyle = def.Foreground(t.theme.errcol)
	t.incorrectSpaceStyle = def.Background(t.theme.errcol)
}

func (t *typer) SetTheme(th theme) {
	t.theme = th
	t.applyStyles()
}

func (t *typer) SetHighlightMode(highlightCurrent, highlightNext bool) {
	t.highlightCurrent = highlightCurrent
	t.highlightNext = highlightNext
	t.applyStyles()
}

func (t *typer) RefreshScreen() {
	t.Scr.SetStyle(t.defaultStyle)
	t.Scr.Clear()
}

func (t *typer) ShowStatusMessage(message string, duration time.Duration) {
	t.statusMessage = message
	if duration <= 0 {
		t.statusMessageUntil = time.Time{}
		return
	}

	t.statusMessageUntil = t.now().Add(duration)
}

func (t *typer) statusMessageActive() string {
	if t.statusMessage == "" {
		return ""
	}

	if !t.statusMessageUntil.IsZero() && t.now().After(t.statusMessageUntil) {
		t.statusMessage = ""
		t.statusMessageUntil = time.Time{}
		return ""
	}

	return t.statusMessage
}

func keyboardHelpDimensions() (width, height int) {
	height = len(keyboardHelpLines)
	for _, line := range keyboardHelpLines {
		if len([]rune(line)) > width {
			width = len([]rune(line))
		}
	}

	return
}

func (t *typer) clearKeyboardHelpArea(sw, sh int) {
	width, height := keyboardHelpDimensions()
	if width > sw {
		width = sw
	}
	if height > sh {
		height = sh
	}

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			t.Scr.SetContent(x, y, ' ', nil, t.defaultStyle)
		}
	}
}

func (t *typer) drawKeyboardHelp(sw, sh int) {
	for y, line := range keyboardHelpLines {
		if y >= sh {
			return
		}

		style := t.defaultStyle
		if y == 0 {
			style = t.currentWordStyle.Bold(true)
		}

		for x, r := range line {
			if x >= sw {
				break
			}
			t.Scr.SetContent(x, y, r, nil, style)
		}
	}
}

// testResult is the outcome of a test.
type testResult struct {
	rc           int
	duration     time.Duration
	correctChars int // Correctly typed characters, used for the speed.
	correctKeys  int // Correct keystrokes, used for the accuracy.
	wrongKeys    int // Wrong keystrokes, used for the accuracy.
	mistakes     []mistake
	keystrokes   []keystroke // Segments are separated by a break.
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

// Start runs a test made of the given segments. A timeout of -1 disables the
// time limit.
func (t *typer) Start(text []segment, timeout time.Duration) (res testResult) {
	timeLeft := timeout

	for i, s := range text {
		r := t.start(s.Text, timeLeft, i > 0, s.Attribution)

		res.rc = r.rc
		res.duration += r.duration
		res.correctChars += r.correctChars
		res.correctKeys += r.correctKeys
		res.wrongKeys += r.wrongKeys
		res.mistakes = append(res.mistakes, r.mistakes...)
		if i > 0 {
			res.keystrokes = append(res.keystrokes, keystroke{})
		}
		res.keystrokes = append(res.keystrokes, r.keystrokes...)

		if timeout != -1 {
			timeLeft -= r.duration
			if timeLeft <= 0 {
				return
			}
		}

		if r.rc != TyperComplete {
			return
		}
	}

	return
}

// start runs the typing of a single segment. The timer starts with the first
// key press, unless startImmediately is set.
func (t *typer) start(text string, timeLimit time.Duration, startImmediately bool, attribution string) testResult {
	s := t.newSession(text, timeLimit, attribution)
	if startImmediately {
		s.startTimer()
	}

	if !t.BlockCursor {
		t.tty.Write([]byte("\033[5 q"))

		//Assumes original cursor shape was a block (the one true cursor shape), there doesn't appear to be a
		//good way to save/restore the shape if the user has changed it from the otcs.
		defer t.tty.Write([]byte("\033[2 q"))
	}

	t.Scr.SetStyle(t.defaultStyle)

	stopTicker := t.startTicker()
	defer stopTicker()

	t.Scr.Clear()
	for {
		s.redraw()

		ev := t.Scr.PollEvent()
		if s.timeUp() { //Whatever happened after the time limit comes too late.
			return s.result()
		}

		switch ev := ev.(type) {
		case *tcell.EventResize:
			return testResult{rc: TyperResize}
		case *tcell.EventKey:
			if res, ended := s.handleKey(ev); ended {
				return res
			}
		default: //The ticks of startTicker only update the screen.
		}
	}
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
			t.Scr.PostEventWait(nil)
		}
	}()

	return func() { close(done) }
}

// refreshInterval returns the interval between screen updates. The pacer
// moves smoothly only if the screen is updated often.
func (t *typer) refreshInterval() time.Duration {
	if t.Pace > 0 && t.tick > paceTick {
		return paceTick
	}

	return t.tick
}

// drawText draws the text of the test at (x, y), styling each character
// according to what was typed. The character at pace is reversed.
func (t *typer) drawText(state *typingState, x, y int, pace int) {
	cx := x
	cy := y
	wordPositions := highlightWordPositions(state.text, state.idx)

	for i, r := range state.text {
		if r == '\n' {
			cy++
			cx = x
			continue
		}

		if i == state.idx {
			t.Scr.ShowCursor(cx, cy)
		}

		var style tcell.Style
		switch {
		case i >= state.idx && wordPositions[i] == 0:
			style = t.currentWordStyle
		case i >= state.idx && wordPositions[i] == 1:
			style = t.nextWordStyle
		case i >= state.idx:
			style = t.defaultStyle
		case t.Blind || r == state.typed[i]:
			style = t.correctStyle
		case r == ' ' || r == '\t':
			style = t.incorrectSpaceStyle
		default:
			style = t.incorrectStyle
		}

		if i == pace {
			style = style.Reverse(true)
		}

		glyph := r
		if r == '\t' {
			glyph = ' '
		}
		for j := 0; j < cellWidth(r); j++ {
			t.Scr.SetContent(cx+j, cy, glyph, nil, style)
		}
		cx += cellWidth(r)
	}
}

// drawLiveStats draws the enabled live statistics above the text.
func (t *typer) drawLiveStats(x, y int, r testResult) {
	if r.duration <= 1e7 { //Avoid flashing large numbers on test start.
		return
	}

	wpmY := y
	if t.ShowAccuracy {
		drawString(t.Scr, x, y, formatLiveAccuracy(r.accuracy()), t.defaultStyle)
		wpmY--
	}

	if t.ShowWpm {
		drawString(t.Scr, x, wpmY, fmt.Sprintf("WPM: %-10d\n", r.wpm()), t.defaultStyle)
	}
}

func formatLiveAccuracy(accuracy float64) string {
	if accuracy == 100 {
		return fmt.Sprintf("Accuracy: %.0f%%\n", accuracy)
	}

	return fmt.Sprintf("Accuracy: %.2f%%\n", accuracy)
}

// drawStatusMessage draws the current status message, if any, centered on
// the second line of the screen.
func (t *typer) drawStatusMessage(sw, sh int) {
	statusY := 0
	if sh > 1 {
		statusY = 1
	}
	for i := 0; i < sw; i++ {
		t.Scr.SetContent(i, statusY, ' ', nil, t.defaultStyle)
	}

	if msg := t.statusMessageActive(); msg != "" {
		msgW, _ := calcStringDimensions(msg)
		msgX := (sw - msgW) / 2
		if msgX < 0 {
			msgX = 0
		}
		drawString(t.Scr, msgX, statusY, msg, t.currentWordStyle.Bold(true))
	}
}

// cycleTheme switches to the next (direction > 0) or previous theme and
// briefly shows its name.
func (t *typer) cycleTheme(direction int) {
	if t.CycleTheme == nil {
		return
	}

	themeName, err := t.CycleTheme(direction)
	if err != nil {
		t.ShowStatusMessage(err.Error(), 2*time.Second)
	} else if themeName != "" {
		t.ShowStatusMessage(fmt.Sprintf("Theme: %s", themeName), 2*time.Second)
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
