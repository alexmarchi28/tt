package main

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
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

type typer struct {
	Scr              tcell.Screen
	SkipWord         bool
	ShowWpm          bool
	ShowAccuracy     bool
	DisableBackspace bool
	BlockCursor      bool
	CycleTheme       func(direction int) (string, error)
	tty              io.Writer

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

	t.statusMessageUntil = time.Now().Add(duration)
}

func (t *typer) statusMessageActive() string {
	if t.statusMessage == "" {
		return ""
	}

	if !t.statusMessageUntil.IsZero() && time.Now().After(t.statusMessageUntil) {
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
}

func (r testResult) cpm() int {
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

func (t *typer) start(s string, timeLimit time.Duration, startImmediately bool, attribution string) testResult {
	var startTime time.Time
	state := newTypingState(s)

	sw, sh := t.Scr.Size()
	nc, nr := calcStringDimensions(s)
	x := (sw - nc) / 2
	y := (sh - nr) / 2

	if !t.BlockCursor {
		t.tty.Write([]byte("\033[5 q"))

		//Assumes original cursor shape was a block (the one true cursor shape), there doesn't appear to be a
		//good way to save/restore the shape if the user has changed it from the otcs.
		defer t.tty.Write([]byte("\033[2 q"))
	}

	t.Scr.SetStyle(t.defaultStyle)

	result := func() testResult {
		return testResult{
			rc:           TyperComplete,
			duration:     time.Since(startTime),
			correctChars: state.correctChars(),
			correctKeys:  state.correctKeys,
			wrongKeys:    state.wrongKeys,
			mistakes:     state.mistakes(),
		}
	}

	startTimer := func() {
		if startTime.IsZero() {
			startTime = time.Now()
		}
	}

	//handleRune types a character and reports whether the text is complete.
	handleRune := func(r rune) bool {
		if !state.done() {
			startTimer()
			state.typeRune(r, t.SkipWord)
		}

		return state.done()
	}

	redraw := func() {
		t.clearKeyboardHelpArea(sw, sh)
		t.drawText(state, x, y)

		aw, ah := calcStringDimensions(attribution)
		drawString(t.Scr, x+nc-aw, y+nr+1, attribution, t.defaultStyle)

		if timeLimit != -1 && !startTime.IsZero() {
			remaining := timeLimit - time.Since(startTime)
			drawString(t.Scr, x+nc/2, y+nr+ah+1, "      ", t.defaultStyle)
			drawString(t.Scr, x+nc/2, y+nr+ah+1, strconv.Itoa(int(remaining/1e9)+1), t.defaultStyle)
		}

		if (t.ShowWpm || t.ShowAccuracy) && !startTime.IsZero() {
			t.drawLiveStats(x+nc/2-4, y-2, result())
		}

		t.drawStatusMessage(sw, sh)

		if t.showKeyboardHelp {
			t.drawKeyboardHelp(sw, sh)
		}

		//Potentially inefficient, but seems to be good enough

		t.Scr.Show()
	}

	tickerCloser := make(chan bool)

	//Inject nil events into the main event loop at regular invervals to force an update
	ticker := func() {
		for {
			select {
			case <-tickerCloser:
				return
			default:
			}

			time.Sleep(time.Duration(5e8))
			t.Scr.PostEventWait(nil)
		}
	}

	go ticker()
	defer close(tickerCloser)

	if startImmediately {
		startTime = time.Now()
	}

	t.Scr.Clear()
	for {
		redraw()

		switch ev := t.Scr.PollEvent().(type) {
		case *tcell.EventResize:
			return testResult{rc: TyperResize}
		case *tcell.EventKey:
			if runtime.GOOS != "windows" && ev.Key() == tcell.KeyBackspace { //Control+backspace on unix terms
				if !t.DisableBackspace {
					startTimer()
					state.deleteWord()
				}
				continue
			}

			switch ev.Key() {
			case tcell.KeyCtrlC:
				return testResult{rc: TyperSigInt}
			case tcell.KeyEscape:
				return testResult{rc: TyperEscape}
			case tcell.KeyRight:
				return testResult{rc: TyperNext}
			case tcell.KeyLeft:
				return testResult{rc: TyperPrevious}
			case tcell.KeyCtrlL:
				t.Scr.Sync()
			case tcell.KeyCtrlN:
				t.cycleTheme(1)
			case tcell.KeyCtrlP:
				t.cycleTheme(-1)
			case tcell.KeyCtrlW:
				if !t.DisableBackspace {
					startTimer()
					state.deleteWord()
				}
			case tcell.KeyBackspace, tcell.KeyBackspace2:
				if !t.DisableBackspace {
					startTimer()
					if ev.Modifiers() == tcell.ModAlt || ev.Modifiers() == tcell.ModCtrl {
						state.deleteWord()
					} else {
						state.backspace()
					}
				}
			case tcell.KeyTab:
				if handleRune('\t') {
					return result()
				}
			case tcell.KeyRune:
				if ev.Rune() == '?' && !state.expects('?') {
					t.showKeyboardHelp = !t.showKeyboardHelp
					continue
				}

				if handleRune(ev.Rune()) {
					return result()
				}
			}
		default: //tick
			if timeLimit != -1 && !startTime.IsZero() && timeLimit <= time.Since(startTime) {
				return result()
			}

			redraw()
		}
	}
}

// drawText draws the text of the test at (x, y), styling each character
// according to what was typed.
func (t *typer) drawText(state *typingState, x, y int) {
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
		case r != state.typed[i] && (r == ' ' || r == '\t'):
			style = t.incorrectSpaceStyle
		case r != state.typed[i]:
			style = t.incorrectStyle
		default:
			style = t.correctStyle
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
