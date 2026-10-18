package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell"
)

// defaultViewOptions are the view options of tt without flags, typed text
// being emboldened so that tests can tell it apart.
var defaultViewOptions = viewOptions{bold: true, highlightCurrent: true, highlightNext: true}

func newTestView(t *testing.T, opts viewOptions) (*view, tcell.SimulationScreen) {
	t.Helper()

	sim := newSimScreen(t)
	sim.SetSize(40, 12)

	return newView(sim, testTheme, opts), sim
}

// typedFrame returns the frame of text drawn at (x, y) after typing typed,
// without skipping words.
func typedFrame(text, typed string, x, y int) frame {
	state := newTypingState(text)
	for _, r := range typed {
		state.typeRune(r, false, 0)
	}

	nc, nr := calcStringDimensions(text)
	return frame{text: state.text, typed: state.typed, idx: state.idx, x: x, y: y, nc: nc, nr: nr, pace: -1}
}

func TestViewDrawsTheText(t *testing.T) {
	v, sim := newTestView(t, defaultViewOptions)

	f := typedFrame("ab cd ef", "xb ", 2, 3)
	f.attribution = "- me"
	v.draw(f)

	if line := screenLine(sim, 3); line != "  ab cd ef" {
		t.Errorf("text line = %q", line)
	}
	if line := screenLine(sim, 5); line != "      - me" {
		t.Errorf("attribution line = %q, want it aligned with the end of the text", line)
	}

	want := []cellWant{
		{testErrCol, testBg, false}, // Mistyped.
		{testHi, testBg, true},      // Typed.
		{testHi, testBg, true},
		{testHi2, testBg, false}, // Current word.
		{testHi2, testBg, false},
		{testFg, testBg, false},
		{testHi3, testBg, false}, // Next word.
	}
	for i, w := range want {
		if fg, bg, bold := cellColors(sim, 2+i, 3); (cellWant{fg, bg, bold}) != w {
			t.Errorf("char %d: got (%v, %v, bold %t), want %+v", i, fg, bg, bold, w)
		}
	}
	if x, y, visible := sim.GetCursor(); x != 5 || y != 3 || !visible {
		t.Errorf("cursor at (%d, %d, visible %t), want (5, 3)", x, y, visible)
	}
}

func TestViewBlind(t *testing.T) {
	v, sim := newTestView(t, viewOptions{blind: true})

	v.draw(typedFrame("ab", "x", 0, 0))

	if fg, _, _ := cellColors(sim, 0, 0); fg != testHi {
		t.Errorf("mistake drawn in %v, want the colour of typed text", fg)
	}
}

func TestViewDrawsThePacer(t *testing.T) {
	v, sim := newTestView(t, defaultViewOptions)

	f := typedFrame("abcdef", "a", 0, 0)
	f.pace = 3
	v.draw(f)

	if got := reversedCells(sim, 0, 0, 6); len(got) != 1 || got[0] != 3 {
		t.Errorf("reversed characters %v, want the pacer at 3", got)
	}
}

func TestViewDrawsTheTimerAndLiveStats(t *testing.T) {
	v, sim := newTestView(t, viewOptions{showWpm: true, showAccuracy: true})

	f := typedFrame("ab cd ef", "ax", 10, 5)
	f.timed, f.timeLeft = true, 9*time.Second+500*time.Millisecond
	f.stats = &testResult{duration: time.Second, correctChars: 1, correctKeys: 1, wrongKeys: 1}
	v.draw(f)

	screen := simulationScreenText(sim)
	for _, want := range []string{"10", "WPM: 12 ", "Accuracy: 50.00%"} {
		if !strings.Contains(screen, want) {
			t.Errorf("screen doesn't show %q:\n%s", want, screen)
		}
	}

	//Live statistics are only shown when requested.
	v, sim = newTestView(t, defaultViewOptions)
	v.draw(f)
	if screen := simulationScreenText(sim); strings.Contains(screen, "WPM") || strings.Contains(screen, "Accuracy") {
		t.Errorf("live statistics shown without being requested:\n%s", screen)
	}
}

func TestViewDrawsTheStatusAndHelp(t *testing.T) {
	v, sim := newTestView(t, defaultViewOptions)

	f := typedFrame("ab", "", 19, 8)
	f.status = "Theme: nord"
	f.help = newKeymap("linux").help()
	v.draw(f)

	if line := screenLine(sim, 1); line != strings.Repeat(" ", 14)+"Theme: nord" {
		t.Errorf("status line = %q, want the status centered", line)
	}

	//The help is drawn over the rest.
	f.showHelp = true
	v.draw(f)
	if line := screenLine(sim, 7); line != "C-n - next theme" {
		t.Errorf("help line = %q", line)
	}

	//The help is erased once hidden.
	f.showHelp = false
	v.draw(f)
	if strings.Contains(simulationScreenText(sim), "next theme") {
		t.Errorf("help still shown:\n%s", simulationScreenText(sim))
	}
}

func TestViewSetTheme(t *testing.T) {
	v, sim := newTestView(t, defaultViewOptions)

	v.setTheme(theme{tcell.ColorAqua, tcell.ColorNavy, tcell.ColorLime, tcell.ColorPurple, tcell.ColorOlive, tcell.ColorMaroon})
	v.draw(typedFrame("ab", "x", 0, 0))

	if fg, bg, _ := cellColors(sim, 0, 0); fg != tcell.ColorMaroon || bg != tcell.ColorNavy {
		t.Errorf("mistake drawn in (%v, %v), want the colours of the new theme", fg, bg)
	}
}

func TestTTYCursor(t *testing.T) {
	var tty bytes.Buffer
	c := ttyCursor{&tty}

	c.typing()
	c.restore()

	if got, want := tty.String(), "\033[5 q\033[2 q"; got != want {
		t.Errorf("the terminal got %q, want a bar cursor then a block cursor", got)
	}
}
