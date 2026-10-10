package main

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell"
)

// readScreen gives fn access to the cells last shown on the screen. The
// simulation screen hands out its internal buffer, so reads are serialised
// with the typer drawing from another goroutine using the screen's own lock.
func readScreen(sim tcell.SimulationScreen, fn func(cells []tcell.SimCell, width, height int)) {
	cells, width, height := sim.GetContents()

	if l, ok := sim.(sync.Locker); ok {
		l.Lock()
		defer l.Unlock()
	}

	fn(cells, width, height)
}

func simulationScreenText(sim tcell.SimulationScreen) string {
	var b strings.Builder

	readScreen(sim, func(cells []tcell.SimCell, width, height int) {
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				cell := cells[y*width+x]
				if len(cell.Runes) == 0 {
					b.WriteRune(' ')
				} else {
					b.WriteRune(cell.Runes[0])
				}
			}
			b.WriteRune('\n')
		}
	})

	return b.String()
}

func waitForScreenText(t *testing.T, sim tcell.SimulationScreen, needle string, want bool) {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(simulationScreenText(sim), needle) == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("screen contains %q = %t, want %t\n%s", needle, !want, want, simulationScreenText(sim))
}

func TestQuestionMarkTogglesKeyboardHelp(t *testing.T) {
	sim := tcell.NewSimulationScreen("")
	if err := sim.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer sim.Fini()

	typer := NewTyper(sim, true, testTheme)
	done := make(chan int, 1)
	go func() {
		done <- typer.start("alpha beta", -1, false, "").rc
	}()

	sim.InjectKey(tcell.KeyRune, '?', tcell.ModNone)
	waitForScreenText(t, sim, "Help", true)
	waitForScreenText(t, sim, "C-n - next theme", true)

	sim.InjectKey(tcell.KeyRune, '?', tcell.ModNone)
	waitForScreenText(t, sim, "Help", false)

	sim.InjectKey(tcell.KeyEscape, 0, tcell.ModNone)
	select {
	case rc := <-done:
		if rc != TyperEscape {
			t.Fatalf("rc = %d, want %d", rc, TyperEscape)
		}
	case <-time.After(time.Second):
		t.Fatal("typer did not exit after escape")
	}
}

var (
	testFg     = tcell.ColorWhite
	testBg     = tcell.ColorBlack
	testHi     = tcell.ColorGreen
	testHi2    = tcell.ColorBlue
	testHi3    = tcell.ColorYellow
	testErrCol = tcell.ColorRed

	testTheme = theme{testFg, testBg, testHi, testHi2, testHi3, testErrCol}
)

func newTestTyper(t *testing.T) (*typer, tcell.SimulationScreen) {
	t.Helper()

	sim := newSimScreen(t)
	return NewTyper(sim, true, testTheme), sim
}

// startTyper runs a test in the background; the outcome is delivered on the
// returned channel once the typer returns.
func startTyper(ty *typer, segments []segment, timeout time.Duration) <-chan testResult {
	done := make(chan testResult, 1)
	go func() { done <- ty.Start(segments, timeout) }()

	return done
}

// keys builds a sequence of key events. Strings are typed rune by rune,
// tcell.Keys are pressed without modifiers and tcell.Events are sent as is.
func keys(items ...interface{}) []tcell.Event {
	var events []tcell.Event

	for _, item := range items {
		switch v := item.(type) {
		case string:
			for _, r := range v {
				events = append(events, tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
			}
		case tcell.Key:
			events = append(events, tcell.NewEventKey(v, 0, tcell.ModNone))
		case tcell.Event:
			events = append(events, v)
		default:
			panic(fmt.Sprintf("unsupported key %T", item))
		}
	}

	return events
}

func sendKeys(t *testing.T, sim tcell.SimulationScreen, items ...interface{}) {
	t.Helper()

	for _, ev := range keys(items...) {
		deadline := time.Now().Add(time.Second)
		for sim.PostEvent(ev) != nil {
			if time.Now().After(deadline) {
				t.Fatal("event queue is full")
			}
			time.Sleep(time.Millisecond)
		}
	}
}

func waitOutcome(t *testing.T, done <-chan testResult) testResult {
	t.Helper()

	select {
	case o := <-done:
		return o
	case <-time.After(5 * time.Second):
		t.Fatal("typer did not return")
	}

	return testResult{}
}

// typeText runs a single segment test with the given keys and returns its
// outcome.
func typeText(t *testing.T, ty *typer, sim tcell.SimulationScreen, text string, items ...interface{}) testResult {
	t.Helper()

	done := startTyper(ty, []segment{{text, ""}}, -1)
	sendKeys(t, sim, items...)
	return waitOutcome(t, done)
}

func assertOutcome(t *testing.T, got testResult, rc, correctChars, correctKeys, wrongKeys int, mistakes []mistake) {
	t.Helper()

	if got.rc != rc {
		t.Errorf("rc = %d, want %d", got.rc, rc)
	}
	if got.correctChars != correctChars {
		t.Errorf("correct characters = %d, want %d", got.correctChars, correctChars)
	}
	if got.correctKeys != correctKeys {
		t.Errorf("correct keystrokes = %d, want %d", got.correctKeys, correctKeys)
	}
	if got.wrongKeys != wrongKeys {
		t.Errorf("wrong keystrokes = %d, want %d", got.wrongKeys, wrongKeys)
	}
	if !reflect.DeepEqual(got.mistakes, mistakes) {
		t.Errorf("mistakes = %v, want %v", got.mistakes, mistakes)
	}
}

func TestTyperCorrectText(t *testing.T) {
	ty, sim := newTestTyper(t)

	got := typeText(t, ty, sim, "alpha beta", "alpha beta")

	// With word skipping enabled spaces are not counted as keystrokes.
	assertOutcome(t, got, TyperComplete, 10, 9, 0, nil)
	if got.duration <= 0 {
		t.Errorf("duration = %v, want > 0", got.duration)
	}
}

func TestTyperTypo(t *testing.T) {
	ty, sim := newTestTyper(t)

	got := typeText(t, ty, sim, "alpha beta", "alpxa beta")
	assertOutcome(t, got, TyperComplete, 9, 8, 1, []mistake{{"alpha", "alpxa"}})
}

func TestTyperBackspace(t *testing.T) {
	ty, sim := newTestTyper(t)

	got := typeText(t, ty, sim, "alpha beta", "alpx", tcell.KeyBackspace2, "ha beta")
	assertOutcome(t, got, TyperComplete, 10, 9, 1, nil)

	got = typeText(t, ty, sim, "ab", tcell.KeyBackspace2, "ab")
	assertOutcome(t, got, TyperComplete, 2, 2, 0, nil)
}

func TestTyperBackspaceAcrossLineBreak(t *testing.T) {
	ty, sim := newTestTyper(t)

	got := typeText(t, ty, sim, "ab \ncd", "ab ", tcell.KeyBackspace2, " cd")
	assertOutcome(t, got, TyperComplete, 5, 4, 0, nil)
}

func TestTyperDisableBackspace(t *testing.T) {
	ty, sim := newTestTyper(t)
	ty.DisableBackspace = true

	got := typeText(t, ty, sim, "ab cd", "x", tcell.KeyBackspace2, "b c", tcell.KeyCtrlW, "d")
	assertOutcome(t, got, TyperComplete, 4, 3, 1, []mistake{{"ab", "xb"}})
}

func TestTyperDeleteWord(t *testing.T) {
	deleteWordKeys := map[string]tcell.Event{
		"C-w":           tcell.NewEventKey(tcell.KeyCtrlW, 0, tcell.ModCtrl),
		"C-backspace":   tcell.NewEventKey(tcell.KeyBackspace, 0, tcell.ModNone),
		"C-backspace2":  tcell.NewEventKey(tcell.KeyBackspace2, 0, tcell.ModCtrl),
		"Alt-backspace": tcell.NewEventKey(tcell.KeyBackspace2, 0, tcell.ModAlt),
	}

	for name, deleteWord := range deleteWordKeys {
		t.Run(name, func(t *testing.T) {
			ty, sim := newTestTyper(t)

			// Inside a word the word itself is deleted.
			got := typeText(t, ty, sim, "alpha beta", "alpha bx", deleteWord, "beta")
			assertOutcome(t, got, TyperComplete, 10, 10, 1, nil)

			// At the start of a word the previous one is deleted.
			got = typeText(t, ty, sim, "alpha beta", "alpxa ", deleteWord, "alpha beta")
			assertOutcome(t, got, TyperComplete, 10, 13, 1, nil)

			// Nothing to delete at the start of the text.
			got = typeText(t, ty, sim, "ab", deleteWord, "ab")
			assertOutcome(t, got, TyperComplete, 2, 2, 0, nil)
		})
	}
}

func TestTyperSpaceSkipsWord(t *testing.T) {
	ty, sim := newTestTyper(t)

	got := typeText(t, ty, sim, "alpha beta gamma", "al beta gamma")
	assertOutcome(t, got, TyperComplete, 13, 11, 0, []mistake{{"alpha", "al___"}})
}

func TestTyperSpaceAtWordStartIsIgnored(t *testing.T) {
	ty, sim := newTestTyper(t)

	got := typeText(t, ty, sim, "alpha beta", "alpha  beta")
	assertOutcome(t, got, TyperComplete, 10, 9, 0, nil)
}

func TestTyperNoSkip(t *testing.T) {
	ty, sim := newTestTyper(t)
	ty.SkipWord = false

	got := typeText(t, ty, sim, "ab cd", "a  cd")
	assertOutcome(t, got, TyperComplete, 4, 4, 1, []mistake{{"ab", "a "}})
}

func TestTyperSkipsLineBreaks(t *testing.T) {
	ty, sim := newTestTyper(t)

	got := typeText(t, ty, sim, "ab\ncd", "abcd")
	assertOutcome(t, got, TyperComplete, 4, 4, 0, nil)

	got = typeText(t, ty, sim, "ab \ncd", "ab cd")
	assertOutcome(t, got, TyperComplete, 5, 4, 0, nil)
}

func TestTyperQuestionMarkInText(t *testing.T) {
	ty, sim := newTestTyper(t)

	got := typeText(t, ty, sim, "why? because", "why? because")
	assertOutcome(t, got, TyperComplete, 12, 11, 0, nil)
}

func TestTyperControlKeys(t *testing.T) {
	tests := []struct {
		name string
		ev   tcell.Event
		rc   int
	}{
		{"escape", tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone), TyperEscape},
		{"ctrl-c", tcell.NewEventKey(tcell.KeyCtrlC, 0, tcell.ModCtrl), TyperSigInt},
		{"right", tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone), TyperNext},
		{"left", tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone), TyperPrevious},
		{"resize", tcell.NewEventResize(100, 30), TyperResize},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ty, sim := newTestTyper(t)

			got := typeText(t, ty, sim, "alpha beta", "al", tc.ev)
			if got.rc != tc.rc {
				t.Errorf("rc = %d, want %d", got.rc, tc.rc)
			}
		})
	}
}

func TestTyperMultipleSegments(t *testing.T) {
	ty, sim := newTestTyper(t)

	done := startTyper(ty, []segment{{"ab cd", ""}, {"ef", ""}}, -1)
	sendKeys(t, sim, "ab cx", "ef")
	got := waitOutcome(t, done)

	if got.rc != TyperComplete || got.correctChars != 6 || !reflect.DeepEqual(got.mistakes, []mistake{{"cd", "cx"}}) {
		t.Errorf("got %+v, want rc %d, 6 correct characters and mistakes [{cd cx}]", got, TyperComplete)
	}

	done = startTyper(ty, []segment{{"ab", ""}, {"cd", ""}}, -1)
	sendKeys(t, sim, "ab", "c", tcell.KeyEscape)
	if got := waitOutcome(t, done); got.rc != TyperEscape {
		t.Errorf("rc = %d, want %d", got.rc, TyperEscape)
	}
}

func TestTyperTimeout(t *testing.T) {
	ty, sim := newTestTyper(t)

	done := startTyper(ty, []segment{{"alpha beta", ""}}, 300*time.Millisecond)
	sendKeys(t, sim, "alx")
	got := waitOutcome(t, done)

	assertOutcome(t, got, TyperComplete, 2, 2, 1, []mistake{{"alp", "alx"}})
	if got.duration < 300*time.Millisecond {
		t.Errorf("duration = %v, want at least the time limit", got.duration)
	}
}

func TestTyperTimeoutSpansSegments(t *testing.T) {
	ty, sim := newTestTyper(t)

	// The time spent on the first segment is subtracted from the time
	// available for the second one. The limit is only checked every 500ms,
	// hence the generous margins.
	timeout := 1500 * time.Millisecond
	done := startTyper(ty, []segment{{"ab", ""}, {"cd ef", ""}}, timeout)
	sendKeys(t, sim, "a")
	time.Sleep(1100 * time.Millisecond)
	sendKeys(t, sim, "b", "c")
	got := waitOutcome(t, done)

	if got.rc != TyperComplete || got.correctChars != 3 {
		t.Errorf("got %+v, want rc %d and 3 correct characters", got, TyperComplete)
	}
	if got.duration < timeout || got.duration > timeout+600*time.Millisecond {
		t.Errorf("duration = %v, want about %v", got.duration, timeout)
	}
}

func TestTyperTimerStartsOnFirstKey(t *testing.T) {
	ty, sim := newTestTyper(t)

	done := startTyper(ty, []segment{{"ab", ""}}, 200*time.Millisecond)
	time.Sleep(700 * time.Millisecond)
	sendKeys(t, sim, "ab")

	assertOutcome(t, waitOutcome(t, done), TyperComplete, 2, 2, 0, nil)
}

// textOrigin returns the screen position of the first character of text.
func textOrigin(t *testing.T, sim tcell.SimulationScreen, text string) (int, int) {
	t.Helper()

	var x, y int
	waitFor(t, sim, func() bool {
		for row, line := range strings.Split(simulationScreenText(sim), "\n") {
			if i := strings.Index(line, text); i != -1 {
				x, y = len([]rune(line[:i])), row
				return true
			}
		}
		return false
	})

	return x, y
}

func waitFor(t *testing.T, sim tcell.SimulationScreen, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("condition not met, screen:\n%s", simulationScreenText(sim))
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func cellColors(sim tcell.SimulationScreen, x, y int) (fg, bg tcell.Color, bold bool) {
	readScreen(sim, func(cells []tcell.SimCell, width, _ int) {
		var attr tcell.AttrMask
		fg, bg, attr = cells[y*width+x].Style.Decompose()
		bold = attr&tcell.AttrBold != 0
	})

	return
}

type cellWant struct {
	fg, bg tcell.Color
	bold   bool
}

// waitForColors waits until the characters of the text drawn at (x, y) have
// the wanted colours, keyed by their offset.
func waitForColors(t *testing.T, sim tcell.SimulationScreen, x, y int, want map[int]cellWant) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for {
		var diff []string
		for i := 0; i < 20; i++ {
			w, ok := want[i]
			if !ok {
				continue
			}

			fg, bg, bold := cellColors(sim, x+i, y)
			if fg != w.fg || bg != w.bg || bold != w.bold {
				diff = append(diff, fmt.Sprintf("char %d: got (%v, %v, bold %t), want (%v, %v, bold %t)", i, fg, bg, bold, w.fg, w.bg, w.bold))
			}
		}

		if len(diff) == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal(strings.Join(diff, "\n"))
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestTyperColors(t *testing.T) {
	ty, sim := newTestTyper(t)
	ty.SkipWord = false

	done := startTyper(ty, []segment{{"ab cd ef", ""}}, -1)
	x, y := textOrigin(t, sim, "ab cd ef")

	def := cellWant{testFg, testBg, false}
	current := cellWant{testHi2, testBg, false}
	next := cellWant{testHi3, testBg, false}
	correct := cellWant{testHi, testBg, true}
	incorrect := cellWant{testErrCol, testBg, false}
	incorrectSpace := cellWant{testFg, testErrCol, false}

	waitForColors(t, sim, x, y, map[int]cellWant{
		0: current, 1: current, 2: def, 3: next, 4: next, 5: def, 6: def, 7: def,
	})

	sendKeys(t, sim, "x")
	waitForColors(t, sim, x, y, map[int]cellWant{
		0: incorrect, 1: current, 2: def, 3: next, 4: next, 5: def, 6: def, 7: def,
	})
	if cx, cy, visible := sim.GetCursor(); cx != x+1 || cy != y || !visible {
		t.Errorf("cursor at (%d, %d, visible %t), want (%d, %d, true)", cx, cy, visible, x+1, y)
	}

	sendKeys(t, sim, "bx")
	waitForColors(t, sim, x, y, map[int]cellWant{
		0: incorrect, 1: correct, 2: incorrectSpace, 3: current, 4: current, 5: def, 6: next, 7: next,
	})

	sendKeys(t, sim, tcell.KeyEscape)
	waitOutcome(t, done)
}

func TestTyperHighlightModes(t *testing.T) {
	tests := []struct {
		name                            string
		highlightCurrent, highlightNext bool
		current, next                   tcell.Color
	}{
		{"both", true, true, testHi2, testHi3},
		{"highlight1", true, false, testHi3, testFg},
		{"highlight2", false, true, testFg, testHi3},
		{"nohighlight", false, false, testFg, testFg},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ty, sim := newTestTyper(t)
			ty.SetHighlightMode(tc.highlightCurrent, tc.highlightNext)

			done := startTyper(ty, []segment{{"ab cd ef", ""}}, -1)
			x, y := textOrigin(t, sim, "ab cd ef")

			waitForColors(t, sim, x, y, map[int]cellWant{
				0: {tc.current, testBg, false},
				3: {tc.next, testBg, false},
				6: {testFg, testBg, false},
			})

			sendKeys(t, sim, tcell.KeyEscape)
			waitOutcome(t, done)
		})
	}
}

func TestTyperSetThemeWhileTyping(t *testing.T) {
	ty, sim := newTestTyper(t)

	ty.CycleTheme = func(direction int) (string, error) {
		ty.SetTheme(theme{tcell.ColorAqua, tcell.ColorNavy, tcell.ColorLime, tcell.ColorPurple, tcell.ColorOlive, tcell.ColorMaroon})
		return "custom", nil
	}

	done := startTyper(ty, []segment{{"ab cd", ""}}, -1)
	x, y := textOrigin(t, sim, "ab cd")

	sendKeys(t, sim, "x", tcell.KeyCtrlN)
	waitForColors(t, sim, x, y, map[int]cellWant{
		0: {tcell.ColorMaroon, tcell.ColorNavy, false},
		1: {tcell.ColorPurple, tcell.ColorNavy, false},
		3: {tcell.ColorOlive, tcell.ColorNavy, false},
	})

	sendKeys(t, sim, tcell.KeyEscape)
	waitOutcome(t, done)
}

func TestTyperCycleTheme(t *testing.T) {
	ty, sim := newTestTyper(t)

	directions := make(chan int, 10)
	ty.CycleTheme = func(direction int) (string, error) {
		directions <- direction
		if direction > 0 {
			return "nord", nil
		}
		return "", fmt.Errorf("no valid themes available")
	}

	done := startTyper(ty, []segment{{"ab", ""}}, -1)

	sendKeys(t, sim, tcell.KeyCtrlN)
	waitForScreenText(t, sim, "Theme: nord", true)

	sendKeys(t, sim, tcell.KeyCtrlP)
	waitForScreenText(t, sim, "no valid themes available", true)

	sendKeys(t, sim, "ab")
	assertOutcome(t, waitOutcome(t, done), TyperComplete, 2, 2, 0, nil)

	close(directions)
	var got []int
	for d := range directions {
		got = append(got, d)
	}
	if !reflect.DeepEqual(got, []int{1, -1}) {
		t.Errorf("CycleTheme called with %v, want [1 -1]", got)
	}
}

func TestTyperShowsAttribution(t *testing.T) {
	ty, sim := newTestTyper(t)

	done := startTyper(ty, []segment{{"some quote text", "- Someone"}}, -1)
	waitForScreenText(t, sim, "- Someone", true)

	sendKeys(t, sim, tcell.KeyEscape)
	waitOutcome(t, done)
}

func TestTyperShowsLiveStats(t *testing.T) {
	ty, sim := newTestTyper(t)
	ty.ShowWpm = true
	ty.ShowAccuracy = true

	done := startTyper(ty, []segment{{"ab cd ef", ""}}, -1)
	waitForScreenText(t, sim, "ab cd ef", true)
	if strings.Contains(simulationScreenText(sim), "WPM:") {
		t.Error("WPM shown before the test started")
	}

	sendKeys(t, sim, "ax")
	waitForScreenText(t, sim, "Accuracy: 50.00%", true)
	waitForScreenText(t, sim, "WPM: ", true)

	sendKeys(t, sim, tcell.KeyEscape)
	waitOutcome(t, done)

	done = startTyper(ty, []segment{{"ab cd ef", ""}}, -1)
	sendKeys(t, sim, "ab")
	waitForScreenText(t, sim, "Accuracy: 100%", true)

	sendKeys(t, sim, tcell.KeyEscape)
	waitOutcome(t, done)
}

func TestTyperCountsKeystrokesOfAllSegments(t *testing.T) {
	ty, sim := newTestTyper(t)

	done := startTyper(ty, []segment{{"ab", ""}, {"cd", ""}}, -1)
	sendKeys(t, sim, "xb", "cd")

	assertOutcome(t, waitOutcome(t, done), TyperComplete, 3, 3, 1, []mistake{{"ab", "xb"}})
}

func TestCalcAccuracy(t *testing.T) {
	tests := []struct {
		correct, errors int
		want            float64
	}{
		{10, 0, 100},
		{3, 1, 75},
		{0, 4, 0},
		{0, 0, 0},
	}

	for _, tc := range tests {
		if got := calcAccuracy(tc.correct, tc.errors); got != tc.want {
			t.Errorf("calcAccuracy(%d, %d) = %v, want %v", tc.correct, tc.errors, got, tc.want)
		}
	}
}

func TestTyperLiveAccuracyWithoutKeystrokes(t *testing.T) {
	ty, sim := newTestTyper(t)
	ty.ShowAccuracy = true

	done := startTyper(ty, []segment{{"ab cd", ""}}, -1)
	sendKeys(t, sim, " ")
	waitForScreenText(t, sim, "Accuracy: 0.00%", true)

	sendKeys(t, sim, tcell.KeyEscape)
	waitOutcome(t, done)
}

func TestTyperSpaceAtLineStartIsIgnored(t *testing.T) {
	ty, sim := newTestTyper(t)

	got := typeText(t, ty, sim, "ab \ncd", "ab  cd")
	assertOutcome(t, got, TyperComplete, 5, 4, 0, nil)

	got = typeText(t, ty, sim, "ab\ncd", "ab cd")
	assertOutcome(t, got, TyperComplete, 4, 4, 0, nil)
}

func TestTyperTypingQuestionMarkDoesNotToggleHelp(t *testing.T) {
	ty, sim := newTestTyper(t)

	done := startTyper(ty, []segment{{"why? because", ""}}, -1)
	x, y := textOrigin(t, sim, "why? because")

	sendKeys(t, sim, "why?")
	waitForColors(t, sim, x, y, map[int]cellWant{3: {testHi, testBg, true}})
	if strings.Contains(simulationScreenText(sim), "C-n - next theme") {
		t.Error("typing a '?' that is part of the text toggled the help")
	}

	sendKeys(t, sim, tcell.KeyEscape)
	waitOutcome(t, done)
}

func TestTestResultStats(t *testing.T) {
	tests := []struct {
		r        testResult
		cpm, wpm int
		accuracy float64
	}{
		{testResult{duration: time.Minute, correctChars: 250, correctKeys: 250}, 250, 50, 100},
		{testResult{duration: 30 * time.Second, correctChars: 251, correctKeys: 3, wrongKeys: 1}, 502, 100, 75},
		{testResult{duration: 90 * time.Second, correctChars: 14}, 9, 1, 0},
	}

	for _, tc := range tests {
		if cpm, wpm, accuracy := tc.r.cpm(), tc.r.wpm(), tc.r.accuracy(); cpm != tc.cpm || wpm != tc.wpm || accuracy != tc.accuracy {
			t.Errorf("%+v: cpm %d, wpm %d, accuracy %v, want %d, %d, %v", tc.r, cpm, wpm, accuracy, tc.cpm, tc.wpm, tc.accuracy)
		}
	}
}
