package main

import (
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/alexmarchi28/tt/internal/typing"
	"github.com/gdamore/tcell"
)

// frame is what the screen shows at a moment of a test, see view.draw.
type frame struct {
	text  []rune
	typed []rune // What was typed at each position of text, see typing.State.
	idx   int    // Position of the cursor in text.

	x, y   int // Position of the text on the screen.
	nc, nr int // Size of the text.

	pace        int // Position of the pacer in text, -1 for none.
	attribution string
	timed       bool          // Whether the time left is shown.
	timeLeft    time.Duration // Time left before the time limit.
	stats       *testResult   // Live statistics, nil before the test starts.

	status   string   // Status message, empty for none.
	help     []string // Lines of the help of the keys.
	showHelp bool
}

// viewOptions say how the text of tests is shown.
type viewOptions struct {
	bold             bool // Embolden typed text.
	highlightCurrent bool // Highlight the current word.
	highlightNext    bool // Highlight the next word.
	blind            bool // Show mistakes like correctly typed characters.
	showWpm          bool
	showAccuracy     bool
}

// view draws the frames of tests on the screen, in the colours of a theme.
type view struct {
	scr  tcell.Screen
	opts viewOptions

	defaultStyle        tcell.Style
	correctStyle        tcell.Style
	currentWordStyle    tcell.Style
	nextWordStyle       tcell.Style
	incorrectStyle      tcell.Style
	incorrectSpaceStyle tcell.Style
}

func newView(scr tcell.Screen, th theme, opts viewOptions) *view {
	v := &view{scr: scr, opts: opts}
	v.setTheme(th)

	return v
}

// setTheme draws the next frames in the colours of th.
func (v *view) setTheme(th theme) {
	def := tcell.StyleDefault.Foreground(th.fgcol).Background(th.bgcol)

	correctStyle := def.Foreground(th.hicol)
	if v.opts.bold {
		correctStyle = correctStyle.Bold(true)
	}

	currentWordStyle := def.Foreground(th.hicol2)
	nextWordStyle := def.Foreground(th.hicol3)
	if !v.opts.highlightNext {
		currentWordStyle = nextWordStyle
		nextWordStyle = def
	}
	if !v.opts.highlightCurrent {
		currentWordStyle = def
	}

	v.defaultStyle = def
	v.correctStyle = correctStyle
	v.currentWordStyle = currentWordStyle
	v.nextWordStyle = nextWordStyle
	v.incorrectStyle = def.Foreground(th.errcol)
	v.incorrectSpaceStyle = def.Background(th.errcol)
}

// clear clears the screen with the background of the theme.
func (v *view) clear() {
	v.scr.SetStyle(v.defaultStyle)
	v.scr.Clear()
}

// sync redraws the whole terminal.
func (v *view) sync() {
	v.scr.Sync()
}

func (v *view) draw(f frame) {
	sw, sh := v.scr.Size()

	v.clearHelpArea(f.help, sw, sh)
	v.drawText(f)

	aw, ah := calcStringDimensions(f.attribution)
	drawString(v.scr, f.x+f.nc-aw, f.y+f.nr+1, f.attribution, v.defaultStyle)

	if f.timed {
		drawString(v.scr, f.x+f.nc/2, f.y+f.nr+ah+1, "      ", v.defaultStyle)
		drawString(v.scr, f.x+f.nc/2, f.y+f.nr+ah+1, strconv.Itoa(int(f.timeLeft/time.Second)+1), v.defaultStyle)
	}

	if f.stats != nil {
		v.drawLiveStats(f.x+f.nc/2-4, f.y-2, *f.stats)
	}

	v.drawStatus(f.status, sw, sh)

	if f.showHelp {
		v.drawHelp(f.help, sw, sh)
	}

	//Potentially inefficient, but seems to be good enough
	v.scr.Show()
}

// drawText draws the text of the test, styling each character according to
// what was typed. The character at the pacer is reversed.
func (v *view) drawText(f frame) {
	cx, cy := f.x, f.y
	wordPositions := typing.WordPositions(f.text, f.idx)

	for i, r := range f.text {
		if r == '\n' {
			cy++
			cx = f.x
			continue
		}

		if i == f.idx {
			v.scr.ShowCursor(cx, cy)
		}

		var style tcell.Style
		switch {
		case i >= f.idx && wordPositions[i] == 0:
			style = v.currentWordStyle
		case i >= f.idx && wordPositions[i] == 1:
			style = v.nextWordStyle
		case i >= f.idx:
			style = v.defaultStyle
		case v.opts.blind || r == f.typed[i]:
			style = v.correctStyle
		case r == ' ' || r == '\t':
			style = v.incorrectSpaceStyle
		default:
			style = v.incorrectStyle
		}

		if i == f.pace {
			style = style.Reverse(true)
		}

		glyph := r
		if r == '\t' {
			glyph = ' '
		}
		for j := 0; j < cellWidth(r); j++ {
			v.scr.SetContent(cx+j, cy, glyph, nil, style)
		}
		cx += cellWidth(r)
	}
}

// drawLiveStats draws the enabled live statistics above the text.
func (v *view) drawLiveStats(x, y int, r testResult) {
	if r.duration <= 1e7 { //Avoid flashing large numbers on test start.
		return
	}

	wpmY := y
	if v.opts.showAccuracy {
		drawString(v.scr, x, y, formatLiveAccuracy(r.accuracy()), v.defaultStyle)
		wpmY--
	}

	if v.opts.showWpm {
		drawString(v.scr, x, wpmY, fmt.Sprintf("WPM: %-10d\n", r.wpm()), v.defaultStyle)
	}
}

func formatLiveAccuracy(accuracy float64) string {
	if accuracy == 100 {
		return fmt.Sprintf("Accuracy: %.0f%%\n", accuracy)
	}

	return fmt.Sprintf("Accuracy: %.2f%%\n", accuracy)
}

// drawStatus draws the status message, if any, centered on the second line
// of the screen.
func (v *view) drawStatus(msg string, sw, sh int) {
	statusY := 0
	if sh > 1 {
		statusY = 1
	}
	for i := 0; i < sw; i++ {
		v.scr.SetContent(i, statusY, ' ', nil, v.defaultStyle)
	}

	if msg != "" {
		msgW, _ := calcStringDimensions(msg)
		msgX := (sw - msgW) / 2
		if msgX < 0 {
			msgX = 0
		}
		drawString(v.scr, msgX, statusY, msg, v.currentWordStyle.Bold(true))
	}
}

// helpDimensions returns the size of the help.
func helpDimensions(help []string) (width, height int) {
	height = len(help)
	for _, line := range help {
		if len([]rune(line)) > width {
			width = len([]rune(line))
		}
	}

	return
}

// clearHelpArea erases the help, which is drawn over the rest of the frame.
func (v *view) clearHelpArea(help []string, sw, sh int) {
	width, height := helpDimensions(help)
	if width > sw {
		width = sw
	}
	if height > sh {
		height = sh
	}

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			v.scr.SetContent(x, y, ' ', nil, v.defaultStyle)
		}
	}
}

func (v *view) drawHelp(help []string, sw, sh int) {
	for y, line := range help {
		if y >= sh {
			return
		}

		style := v.defaultStyle
		if y == 0 {
			style = v.currentWordStyle.Bold(true)
		}

		for x, r := range line {
			if x >= sw {
				break
			}
			v.scr.SetContent(x, y, r, nil, style)
		}
	}
}

// cursorShaper sets the shape of the terminal cursor while typing.
type cursorShaper interface {
	typing()  // Uses the cursor shape of typing.
	restore() // Restores the cursor shape.
}

// ttyCursor shows a bar cursor while typing by writing escape sequences to
// the terminal.
type ttyCursor struct{ tty io.Writer }

func (c ttyCursor) typing() { io.WriteString(c.tty, "\033[5 q") }

// restore assumes the original cursor shape was a block (the one true cursor
// shape), there doesn't appear to be a good way to save/restore the shape if
// the user has changed it from the otcs.
func (c ttyCursor) restore() { io.WriteString(c.tty, "\033[2 q") }

// keptCursor leaves the cursor shape as it is.
type keptCursor struct{}

func (keptCursor) typing()  {}
func (keptCursor) restore() {}
